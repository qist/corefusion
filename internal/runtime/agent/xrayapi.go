package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"corefusion/internal/coredata"
	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	statsCommand "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/infra/conf"
	hyAccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"

	"corefusion/internal/runtime/firewall"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// XrayAPIProxy handles generic xray API calls by proxying them to the local
// core's gRPC API. This is used by the Agent to serve xray API requests
// from the panel for remote nodes.
//
// For xray: connects to xray gRPC API (default 127.0.0.1:10085)
// For sing-box: connects to sing-box V2Ray API (experimental.v2ray_api.listen)
//
// sing-box's V2Ray API is protocol-compatible with xray's StatsService,
// so the same gRPC client (statsCommand.NewStatsServiceClient) works.
// However, sing-box does not implement HandlerService (proxyman) or
// RoutingService — those requests will fail gracefully.
func (a *LocalAgent) XrayAPIProxy(ctx context.Context, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	// For stats service: try xray first, then sing-box (both use same gRPC protobuf)
	if req.Service == "stats" {
		return a.proxyStatsAny(ctx, req)
	}

	// For proxyman/router/firewall: only xray supports these
	conn, err := a.getXrayConn()
	if err != nil {
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("connect to xray: %v", err)}, nil
	}

	switch req.Service {
	case "proxyman":
		return a.proxyProxyman(ctx, conn, req)
	case "router":
		return a.proxyRouter(ctx, conn, req)
	case "firewall":
		return a.proxyFirewall(ctx, req)
	default:
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unknown service: %s", req.Service)}, nil
	}
}

// getXrayConn returns a persistent gRPC connection to the local xray API.
func (a *LocalAgent) getXrayConn() (*grpc.ClientConn, error) {
	a.xrayConnMu.Lock()
	defer a.xrayConnMu.Unlock()

	if a.xrayClient == nil {
		a.xrayClient = coredata.NewXrayClient(a.statsAddr)
	}
	conn, err := a.xrayClient.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("connect to xray: %w", err)
	}
	a.xrayConn = conn
	return conn, nil
}

// ============================================================
// proxyman service proxy
// ============================================================

func (a *LocalAgent) proxyProxyman(ctx context.Context, conn *grpc.ClientConn, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	client := proxymanCommand.NewHandlerServiceClient(conn)

	switch req.Method {
	case "addInbound":
		var payload struct {
			Config json.RawMessage `json:"config"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		// Rewrite certificate paths: the panel sends absolute paths like
		// /opt/corefusion/data/certs/gg/cert.pem which don't exist on the
		// Agent's filesystem. We rewrite them to the Agent's local dataDir.
		rewrittenConfig := rewriteCertPaths(payload.Config, a.dataDir)
		inboundConf := new(conf.InboundDetourConfig)
		if err := json.Unmarshal(rewrittenConfig, inboundConf); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse inbound config: %v", err)}, nil
		}
		config, err := inboundConf.Build()
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("build inbound: %v", err)}, nil
		}
		_, err = client.AddInbound(ctx, &proxymanCommand.AddInboundRequest{Inbound: config})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"inbound added"}`)}, nil

	case "removeInbound":
		var payload struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := client.RemoveInbound(ctx, &proxymanCommand.RemoveInboundRequest{Tag: payload.Tag})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"inbound removed"}`)}, nil

	case "addOutbound":
		var payload struct {
			Config json.RawMessage `json:"config"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		outboundConf := new(conf.OutboundDetourConfig)
		if err := json.Unmarshal(payload.Config, outboundConf); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse outbound config: %v", err)}, nil
		}
		config, err := outboundConf.Build()
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("build outbound: %v", err)}, nil
		}
		_, err = client.AddOutbound(ctx, &proxymanCommand.AddOutboundRequest{Outbound: config})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"outbound added"}`)}, nil

	case "removeOutbound":
		var payload struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := client.RemoveOutbound(ctx, &proxymanCommand.RemoveOutboundRequest{Tag: payload.Tag})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"outbound removed"}`)}, nil

	case "addUser":
		var payload struct {
			Tag      string `json:"tag"`
			Email    string `json:"email"`
			ID       string `json:"id"`
			Password string `json:"password"`
			Flow     string `json:"flow"`
			Method   string `json:"method"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		account := buildXrayAccountFromParams(payload.Tag, payload.Email, payload.ID, payload.Password, payload.Flow, payload.Method)
		if account == nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unsupported protocol for tag: %s", payload.Tag)}, nil
		}
		user := &protocol.User{Email: payload.Email, Account: account}
		_, err := client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
			Tag:       payload.Tag,
			Operation: serial.ToTypedMessage(&proxymanCommand.AddUserOperation{User: user}),
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		resp, _ := json.Marshal(map[string]string{"message": "user added", "email": payload.Email, "tag": payload.Tag})
		return &agentpb.XrayAPIResponse{Success: true, Response: resp}, nil

	case "removeUser":
		var payload struct {
			Tag   string `json:"tag"`
			Email string `json:"email"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
			Tag:       payload.Tag,
			Operation: serial.ToTypedMessage(&proxymanCommand.RemoveUserOperation{Email: payload.Email}),
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		resp, _ := json.Marshal(map[string]string{"message": "user removed", "email": payload.Email, "tag": payload.Tag})
		return &agentpb.XrayAPIResponse{Success: true, Response: resp}, nil

	default:
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unknown proxyman method: %s", req.Method)}, nil
	}
}

// ============================================================
// router service proxy
// ============================================================

func (a *LocalAgent) proxyRouter(ctx context.Context, conn *grpc.ClientConn, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	client := routerCommand.NewRoutingServiceClient(conn)

	switch req.Method {
	case "applyRouting":
		var payload struct {
			Routing json.RawMessage `json:"routing"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		routerConf := new(conf.RouterConfig)
		if err := json.Unmarshal(payload.Routing, routerConf); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse routing: %v", err)}, nil
		}
		ensureImplicitAPIRoutingRuleLocal(routerConf)
		config, err := routerConf.Build()
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("build routing: %v", err)}, nil
		}
		_, err = client.AddRule(ctx, &routerCommand.AddRuleRequest{
			ShouldAppend: false,
			Config:       serial.ToTypedMessage(config),
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"routing applied"}`)}, nil

	case "addRoutingRule":
		var payload struct {
			Rule   json.RawMessage `json:"rule"`
			Append bool            `json:"append"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		routerConf := &conf.RouterConfig{RuleList: []json.RawMessage{payload.Rule}}
		config, err := routerConf.Build()
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("build rule: %v", err)}, nil
		}
		_, err = client.AddRule(ctx, &routerCommand.AddRuleRequest{
			ShouldAppend: payload.Append,
			Config:       serial.ToTypedMessage(config),
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"routing rule added"}`)}, nil

	case "removeRoutingRule":
		var payload struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := client.RemoveRule(ctx, &routerCommand.RemoveRuleRequest{RuleTag: payload.Tag})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"routing rule removed"}`)}, nil

	case "listRoutingRules":
		resp, err := client.ListRule(ctx, &routerCommand.ListRuleRequest{})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		rules := make([]map[string]interface{}, 0)
		for _, rule := range resp.GetRules() {
			rules = append(rules, map[string]interface{}{
				"tag":     rule.GetTag(),
				"ruleTag": rule.GetRuleTag(),
			})
		}
		respJSON, _ := json.Marshal(rules)
		return &agentpb.XrayAPIResponse{Success: true, Response: respJSON}, nil

	case "getBalancerInfo":
		var payload struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		resp, err := client.GetBalancerInfo(ctx, &routerCommand.GetBalancerInfoRequest{Tag: payload.Tag})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		info := map[string]interface{}{"tag": payload.Tag}
		if balancer := resp.GetBalancer(); balancer != nil {
			if balancer.Override != nil {
				info["override"] = balancer.Override.Target
			}
			if balancer.PrincipleTarget != nil {
				info["selected"] = balancer.PrincipleTarget.Tag
			}
		}
		respJSON, _ := json.Marshal(info)
		return &agentpb.XrayAPIResponse{Success: true, Response: respJSON}, nil

	case "setBalancerTarget":
		var payload struct {
			Tag    string `json:"tag"`
			Target string `json:"target"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := client.OverrideBalancerTarget(ctx, &routerCommand.OverrideBalancerTargetRequest{
			BalancerTag: payload.Tag,
			Target:      payload.Target,
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"balancer target set"}`)}, nil

	default:
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unknown router method: %s", req.Method)}, nil
	}
}

// ============================================================
// stats service proxy
// ============================================================

// proxyStatsAny tries xray first, then sing-box, for stats requests.
// This is needed because sing-box's V2Ray API uses a different listen
// address (experimental.v2ray_api.listen, default 127.0.0.1:8080) but
// the same gRPC StatsService protobuf.
//
// For "getStats" and "getUserStats": tries xray first; if connection fails,
// tries sing-box. If both succeed, merges results.
// For "resetStats" and "resetUserStats": tries xray first, then sing-box.
func (a *LocalAgent) proxyStatsAny(ctx context.Context, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	// Try xray connection
	xrayConn, xrayErr := a.getXrayConn()
	xrayResp := (*agentpb.XrayAPIResponse)(nil)
	if xrayErr == nil {
		xrayResp, _ = a.proxyStats(ctx, xrayConn, req, false)
	}

	// Try sing-box connection (V2Ray API)
	singboxResp := (*agentpb.XrayAPIResponse)(nil)
	sbConn, sbErr := a.getSingboxV2RayConn()
	if sbErr == nil {
		singboxResp, _ = a.proxyStats(ctx, sbConn, req, true)
	}

	// If neither worked
	if xrayResp == nil && singboxResp == nil {
		return &agentpb.XrayAPIResponse{
			Success: false,
			Error:   fmt.Sprintf("connect failed: xray(%v), singbox(%v)", xrayErr, sbErr),
		}, nil
	}

	// For reset operations, success from either is fine
	if req.Method == "resetStats" || req.Method == "resetUserStats" {
		if xrayResp != nil && xrayResp.Success {
			return xrayResp, nil
		}
		return singboxResp, nil
	}

	// For query operations, merge results from both cores
	// If only one succeeded, return that one
	if xrayResp == nil || !xrayResp.Success {
		return singboxResp, nil
	}
	if singboxResp == nil || !singboxResp.Success {
		return xrayResp, nil
	}

	// Both succeeded — merge user and inbound stats
	return mergeStatsResponses(xrayResp, singboxResp)
}

// getSingboxV2RayConn returns a gRPC connection to the local sing-box V2Ray API.
// The address is resolved from the sing-box config's experimental.v2ray_api.listen
// field, falling back to the same statsAddr as xray (which may be configured
// to point to sing-box), then to the default sing-box V2Ray API address.
func (a *LocalAgent) getSingboxV2RayConn() (*grpc.ClientConn, error) {
	a.xrayConnMu.Lock()
	defer a.xrayConnMu.Unlock()

	// Determine sing-box V2Ray API address
	// Priority: singboxStatsAddr (if set) > sing-box config v2ray_api.listen > default
	addr := a.singboxStatsAddr
	if addr == "" {
		addr = coredata.DefaultSingboxStatsAddr // sing-box V2Ray API 默认 127.0.0.1:10086
	}

	// 复用持久客户端：每次 stats 查询新建连接会泄漏（metrics 每 30s 一轮），
	// 最终把 sing-box 的 gRPC 连接数堆到上千条。地址变化时重建。
	if a.singboxClient == nil || a.singboxClient.Addr() != addr {
		if a.singboxClient != nil {
			a.singboxClient.Close()
		}
		a.singboxClient = coredata.NewV2RayClient(addr)
	}
	conn, err := a.singboxClient.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("connect to sing-box v2ray_api %s: %w", addr, err)
	}
	return conn, nil
}

// mergeStatsResponses merges two XrayAPIResponse stats results into one.
// Both responses contain JSON with "users" and "inbounds" arrays.
func mergeStatsResponses(responses ...*agentpb.XrayAPIResponse) (*agentpb.XrayAPIResponse, error) {
	mergedUsers := make(map[string]map[string]int64) // email -> {uplink, downlink}
	mergedInbounds := make(map[string]map[string]int64)

	for _, resp := range responses {
		if resp == nil || !resp.Success {
			continue
		}
		var data struct {
			Users    []map[string]interface{} `json:"users"`
			Inbounds []map[string]interface{} `json:"inbounds"`
		}
		if err := json.Unmarshal(resp.Response, &data); err != nil {
			continue
		}
		for _, u := range data.Users {
			email, _ := u["email"].(string)
			if email == "" {
				continue
			}
			if mergedUsers[email] == nil {
				mergedUsers[email] = map[string]int64{}
			}
			up, _ := toInt64(u["uplink"])
			down, _ := toInt64(u["downlink"])
			mergedUsers[email]["uplink"] += up
			mergedUsers[email]["downlink"] += down
		}
		for _, ib := range data.Inbounds {
			tag, _ := ib["tag"].(string)
			if tag == "" {
				continue
			}
			if mergedInbounds[tag] == nil {
				mergedInbounds[tag] = map[string]int64{}
			}
			up, _ := toInt64(ib["uplink"])
			down, _ := toInt64(ib["downlink"])
			mergedInbounds[tag]["uplink"] += up
			mergedInbounds[tag]["downlink"] += down
		}
	}

	// Build merged output
	users := make([]map[string]interface{}, 0, len(mergedUsers))
	for email, v := range mergedUsers {
		users = append(users, map[string]interface{}{
			"email":    email,
			"uplink":   v["uplink"],
			"downlink": v["downlink"],
			"total":    v["uplink"] + v["downlink"],
		})
	}
	inbounds := make([]map[string]interface{}, 0, len(mergedInbounds))
	for tag, v := range mergedInbounds {
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      tag,
			"uplink":   v["uplink"],
			"downlink": v["downlink"],
			"total":    v["uplink"] + v["downlink"],
		})
	}

	result := map[string]interface{}{
		"users":    users,
		"inbounds": inbounds,
	}
	respJSON, _ := json.Marshal(result)
	return &agentpb.XrayAPIResponse{Success: true, Response: respJSON}, nil
}

// toInt64 converts an interface{} to int64 (handles float64, int, int64, json.Number).
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

// proxyStats 通过给定连接代理 stats 请求。
//
// singbox=true 时必须以 sing-box 注册的服务名
// （v2ray.core.app.stats.command.StatsService）调用：sing-box 与 xray 的 stats
// protobuf 字段一致但服务名不同，用 xray 的客户端查 sing-box 会一直得到
// Unimplemented，而调用方往往「只用一个成功的结果」，于是静默退化成只有 xray 数据。
func (a *LocalAgent) proxyStats(ctx context.Context, conn *grpc.ClientConn, req *agentpb.XrayAPIRequest, singbox bool) (*agentpb.XrayAPIResponse, error) {
	queryStats := func(r *statsCommand.QueryStatsRequest) (*statsCommand.QueryStatsResponse, error) {
		if singbox {
			resp := &statsCommand.QueryStatsResponse{}
			if err := conn.Invoke(ctx, coredata.SingboxStatsMethod("QueryStats"), r, resp); err != nil {
				return nil, err
			}
			return resp, nil
		}
		return statsCommand.NewStatsServiceClient(conn).QueryStats(ctx, r)
	}

	switch req.Method {
	case "getStats":
		resp, err := queryStats(&statsCommand.QueryStatsRequest{Reset_: false})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return buildStatsResponse(resp)

	case "resetStats":
		_, err := queryStats(&statsCommand.QueryStatsRequest{Reset_: true})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"stats reset"}`)}, nil

	case "getUserStats":
		var payload struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		resp, err := queryStats(&statsCommand.QueryStatsRequest{
			Pattern: fmt.Sprintf("user>>>%s>>>traffic>>>", payload.Email),
			Reset_:  false,
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return buildStatsResponse(resp)

	case "resetUserStats":
		var payload struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		_, err := queryStats(&statsCommand.QueryStatsRequest{
			Pattern: fmt.Sprintf("user>>>%s>>>traffic>>>", payload.Email),
			Reset_:  true,
		})
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"user stats reset"}`)}, nil

	default:
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unknown stats method: %s", req.Method)}, nil
	}
}

// buildStatsResponse converts xray stats response to JSON.
// 解析逻辑复用 coredata.ParseStatsResponse，避免与面板侧/采集器重复实现。
func buildStatsResponse(resp *statsCommand.QueryStatsResponse) (*agentpb.XrayAPIResponse, error) {
	sr, err := coredata.ParseStatsResponse(resp)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"users":    sr.Users,
		"inbounds": sr.Inbounds,
	}
	respJSON, _ := json.Marshal(result)
	return &agentpb.XrayAPIResponse{Success: true, Response: respJSON}, nil
}

// buildXrayAccountFromParams builds a xray account message from user parameters.
// This mirrors the logic in buildXrayAccount (local.go) but takes individual params.
func buildXrayAccountFromParams(tag, email, id, password, flow, method string) *serial.TypedMessage {
	tagLower := strings.ToLower(tag)

	if strings.Contains(tagLower, "vless") {
		account := &vless.Account{
			Id:   id,
			Flow: flow,
		}
		return serial.ToTypedMessage(account)
	}

	if strings.Contains(tagLower, "vmess") {
		account := &vmess.Account{
			Id: id,
		}
		return serial.ToTypedMessage(account)
	}

	if strings.Contains(tagLower, "trojan") {
		account := &trojan.Account{
			Password: password,
		}
		return serial.ToTypedMessage(account)
	}

	if strings.Contains(tagLower, "shadowsocks-2022") || strings.Contains(tagLower, "ss2022") {
		account := &ss2022.Account{
			Key: password,
		}
		return serial.ToTypedMessage(account)
	}

	if strings.Contains(tagLower, "shadowsocks") || strings.Contains(tagLower, "ss-") {
		if strings.HasPrefix(method, "2022-blake3") {
			account := &ss2022.Account{
				Key: password,
			}
			return serial.ToTypedMessage(account)
		}
		cipherType := shadowsocks.CipherType_AES_256_GCM
		switch method {
		case "aes-128-gcm":
			cipherType = shadowsocks.CipherType_AES_128_GCM
		case "chacha20-poly1305":
			cipherType = shadowsocks.CipherType_CHACHA20_POLY1305
		case "xchacha20-ietf-poly1305":
			cipherType = shadowsocks.CipherType_XCHACHA20_POLY1305
		case "none":
			cipherType = shadowsocks.CipherType_UNKNOWN
		}
		account := &shadowsocks.Account{
			Password:   password,
			CipherType: cipherType,
		}
		return serial.ToTypedMessage(account)
	}

	if strings.Contains(tagLower, "hysteria") {
		account := &hyAccount.Account{
			Auth: password,
		}
		return serial.ToTypedMessage(account)
	}

	return nil
}

// ensureImplicitAPIRoutingRuleLocal ensures the API routing rule exists.
func ensureImplicitAPIRoutingRuleLocal(routerConf *conf.RouterConfig) {
	if routerConf == nil {
		return
	}
	for _, rawRule := range routerConf.RuleList {
		var rule struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
		}
		if err := json.Unmarshal(rawRule, &rule); err != nil {
			continue
		}
		if rule.OutboundTag != "api" {
			continue
		}
		for _, inboundTag := range rule.InboundTag {
			if inboundTag == "api" {
				return
			}
		}
	}

	apiRule := json.RawMessage(`{"type":"field","inboundTag":["api"],"outboundTag":"api"}`)
	routerConf.RuleList = append([]json.RawMessage{apiRule}, routerConf.RuleList...)
}

// ============================================================
// firewall service proxy (nftables/iptables on Agent host)
// ============================================================

func (a *LocalAgent) proxyFirewall(ctx context.Context, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	switch req.Method {
	case "apply":
		var payload struct {
			PortRange  string `json:"port_range"`
			TargetPort int    `json:"target_port"`
			Interface  string `json:"interface"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		if payload.PortRange == "" || payload.TargetPort == 0 {
			return &agentpb.XrayAPIResponse{Success: false, Error: "port_range and target_port are required"}, nil
		}
		useNft := isNftablesAvailable()
		exists, err := checkFirewallRuleExists(useNft, payload.PortRange, payload.TargetPort, payload.Interface)
		if err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		if exists {
			ftype := "iptables"
			if useNft {
				ftype = "nftables"
			}
			resp, _ := json.Marshal(map[string]string{"message": "rule already exists", "type": ftype})
			return &agentpb.XrayAPIResponse{Success: true, Response: resp}, nil
		}
		// 端口段是节点级全局资源：同一段只能指向一个入站端口，
		// 否则后加的规则不生效、入站静默不可达（xray 与 sing-box 可能同时开）。
		if err := firewall.CheckConflict(firewall.RuleSpec{
			PortRange:  payload.PortRange,
			TargetPort: payload.TargetPort,
			Interface:  payload.Interface,
		}); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		if err := applyFirewallRule(useNft, payload.PortRange, payload.TargetPort, payload.Interface); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		ftype := "iptables"
		if useNft {
			ftype = "nftables"
		}
		resp, _ := json.Marshal(map[string]string{"message": "firewall rule applied", "type": ftype})
		return &agentpb.XrayAPIResponse{Success: true, Response: resp}, nil

	case "remove":
		var payload struct {
			PortRange  string `json:"port_range"`
			TargetPort int    `json:"target_port"`
			Interface  string `json:"interface"`
		}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("parse payload: %v", err)}, nil
		}
		if payload.PortRange == "" || payload.TargetPort == 0 {
			return &agentpb.XrayAPIResponse{Success: false, Error: "port_range and target_port are required"}, nil
		}
		useNft := isNftablesAvailable()
		if err := removeFirewallRule(useNft, payload.PortRange, payload.TargetPort, payload.Interface); err != nil {
			return &agentpb.XrayAPIResponse{Success: false, Error: err.Error()}, nil
		}
		ftype := "iptables"
		if useNft {
			ftype = "nftables"
		}
		resp, _ := json.Marshal(map[string]string{"message": "firewall rule removed", "type": ftype})
		return &agentpb.XrayAPIResponse{Success: true, Response: resp}, nil

	default:
		return &agentpb.XrayAPIResponse{Success: false, Error: fmt.Sprintf("unknown firewall method: %s", req.Method)}, nil
	}
}

// Firewall helper functions (mirroring xraycontrol/firewall.go)
func isNftablesAvailable() bool {
	_, err := exec.LookPath("nft")
	return err == nil
}

func checkFirewallRuleExists(useNft bool, portRange string, targetPort int, iface string) (bool, error) {
	if useNft {
		return checkNftRuleExists(portRange, targetPort, iface)
	}
	return checkIptablesRuleExists(portRange, targetPort, iface)
}

func checkNftRuleExists(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("nft", "list", "table", "inet", "hysteria_porthopping")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	handle, err := findNftRuleHandle(portRange, targetPort, iface)
	if err != nil {
		return false, err
	}
	return handle > 0, nil
}

// nftRuleMatches checks if a rule line from `nft -a list chain` output matches
// the given port range, target port, and interface.
func nftRuleMatches(line, portRange string, targetPort int, iface string) bool {
	if !strings.Contains(line, "redirect to") {
		return false
	}
	if !strings.Contains(line, fmt.Sprintf("dport %s ", portRange)) {
		return false
	}
	if !strings.Contains(line, fmt.Sprintf("redirect to :%d", targetPort)) {
		return false
	}
	if iface != "" {
		if !strings.Contains(line, fmt.Sprintf(`iifname "%s"`, iface)) {
			return false
		}
	} else {
		if strings.Contains(line, "iifname ") {
			return false
		}
	}
	return true
}

// findNftRuleHandle lists the chain with handles and finds the handle of the
// rule matching the given spec. Returns 0 if not found.
func findNftRuleHandle(portRange string, targetPort int, iface string) (uint64, error) {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", "hysteria_porthopping", "prerouting")
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !nftRuleMatches(line, portRange, targetPort, iface) {
			continue
		}
		handleIdx := strings.LastIndex(line, "handle ")
		if handleIdx < 0 {
			continue
		}
		handleStr := strings.TrimSpace(line[handleIdx+7:])
		handle, err := strconv.ParseUint(handleStr, 10, 64)
		if err != nil {
			continue
		}
		return handle, nil
	}
	return 0, nil
}

func checkIptablesRuleExists(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	rules := string(out)
	if strings.Contains(rules, portRange) && strings.Contains(rules, fmt.Sprintf("to:%d", targetPort)) {
		return true, nil
	}
	return false, nil
}

func applyFirewallRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return applyNftRule(portRange, targetPort, iface)
	}
	return applyIptablesRule(portRange, targetPort, iface)
}

func applyNftRule(portRange string, targetPort int, iface string) error {
	exec.Command("nft", "add", "table", "inet", "hysteria_porthopping").Run()
	exec.Command("nft", "add", "chain", "inet", "hysteria_porthopping", "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }").Run()
	rule := fmt.Sprintf("udp dport %s counter redirect to :%d", portRange, targetPort)
	if iface != "" {
		rule = fmt.Sprintf("iifname %s %s", iface, rule)
	}
	return exec.Command("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule).Run()
}

func applyIptablesRule(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-A", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))
	return exec.Command("iptables", args...).Run()
}

func removeFirewallRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return removeNftRule(portRange, targetPort, iface)
	}
	return removeIptablesRule(portRange, targetPort, iface)
}

func removeNftRule(portRange string, targetPort int, iface string) error {
	handle, err := findNftRuleHandle(portRange, targetPort, iface)
	if err != nil {
		return fmt.Errorf("find rule handle: %w", err)
	}
	if handle == 0 {
		return nil // rule doesn't exist, nothing to remove
	}
	return exec.Command("nft", "delete", "rule", "inet", "hysteria_porthopping", "prerouting", "handle", strconv.FormatUint(handle, 10)).Run()
}

func removeIptablesRule(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-D", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))
	return exec.Command("iptables", args...).Run()
}

// Helper functions (mirroring xraycontrol handler helpers)
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func splitN(s, sep string) []string {
	return strings.Split(s, sep)
}

// rewriteCertPaths rewrites certificate/key file paths in an inbound config JSON.
// The panel sends absolute paths like /opt/corefusion/data/certs/gg/cert.pem,
// which don't exist on the Agent's filesystem. This function rewrites them
// to use the Agent's local dataDir.
//
// It handles paths in:
//   - streamSettings.tlsSettings.certificates[].certificateFile
//   - streamSettings.tlsSettings.certificates[].keyFile
//   - streamSettings.tlsSettings.certificates[].certificate (content, not path — skipped)
//
// Rewriting rules:
//   - Absolute path containing "certs/" → extract from "certs/" onward, prepend dataDir
//   - Relative path with ../../../ prefix → strip prefix, prepend dataDir
//   - Relative path without prefix → prepend dataDir
//   - Empty or content-based → leave as-is
func rewriteCertPaths(configJSON json.RawMessage, dataDir string) json.RawMessage {
	if len(configJSON) == 0 || dataDir == "" {
		return configJSON
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return configJSON // can't parse, return as-is
	}

	ss, ok := cfg["streamSettings"].(map[string]interface{})
	if !ok {
		return configJSON
	}
	tls, ok := ss["tlsSettings"].(map[string]interface{})
	if !ok {
		return configJSON
	}
	certs, ok := tls["certificates"].([]interface{})
	if !ok {
		return configJSON
	}

	for _, c := range certs {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if certFile, ok := cm["certificateFile"].(string); ok && certFile != "" {
			cm["certificateFile"] = rewritePathToLocal(certFile, dataDir)
		}
		if keyFile, ok := cm["keyFile"].(string); ok && keyFile != "" {
			cm["keyFile"] = rewritePathToLocal(keyFile, dataDir)
		}
	}

	result, err := json.Marshal(cfg)
	if err != nil {
		return configJSON
	}
	return result
}

// rewritePathToLocal converts a certificate path from the panel's filesystem
// to the Agent's local filesystem.
//
// Examples:
//
//	/opt/corefusion/data/certs/gg/cert.pem → /opt/corefusion-agent/data/certs/gg/cert.pem
//	../../../certs/gg/cert.pem             → /opt/corefusion-agent/data/certs/gg/cert.pem
//	certs/gg/cert.pem                      → /opt/corefusion-agent/data/certs/gg/cert.pem
func rewritePathToLocal(path, dataDir string) string {
	// Find "certs/" in the path — everything from there is the relative part
	idx := strings.Index(path, "certs/")
	if idx >= 0 {
		relPath := path[idx:]
		return dataDir + "/" + relPath
	}
	// If no "certs/" found, return as-is (might be a non-cert path)
	return path
}
