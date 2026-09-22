package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"corefusion/internal/model/entity"
	"corefusion/pkg/netutil"

	"corefusion/internal/store/repository"
	"corefusion/internal/subscription/mihomoconv"
)

// ExternalNodeHandler handles external node source and node API requests.
type ExternalNodeHandler struct {
	sourceRepo *repository.ExternalNodeSourceRepository
	nodeRepo   *repository.ExternalNodeRepository
}

func NewExternalNodeHandler(sourceRepo *repository.ExternalNodeSourceRepository, nodeRepo *repository.ExternalNodeRepository) *ExternalNodeHandler {
	return &ExternalNodeHandler{sourceRepo: sourceRepo, nodeRepo: nodeRepo}
}

func (h *ExternalNodeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/external-nodes")
	g.GET("/sources", h.ListSources)
	g.POST("/sources", h.CreateSource)
	g.PUT("/sources/:id", h.UpdateSource)
	g.DELETE("/sources/:id", h.DeleteSource)
	g.POST("/sources/parse", h.Parse)

	g.GET("", h.ListNodes)
	g.GET("/:id", h.GetNode)
	g.PUT("/:id", h.UpdateNode)
	g.DELETE("/:id", h.DeleteNode)
}

// --- Source CRUD ---

func (h *ExternalNodeHandler) ListSources(c *gin.Context) {
	ctx := c.Request.Context()
	sources, err := h.sourceRepo.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sources})
}

func (h *ExternalNodeHandler) CreateSource(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		Name       string `json:"name"`
		SourceType string `json:"source_type"`
		SourceData string `json:"source_data"`
		SourceURL  string `json:"source_url"`
		Enabled    *bool  `json:"enabled"`
		SortOrder  int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.SourceType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and source_type required"})
		return
	}

	// Parse nodes
	proxyMaps, err := parseToProxyMaps(req.SourceType, req.SourceData, req.SourceURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse error: %v", err)})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Create source
	source := &entity.ExternalNodeSource{
		Name:       req.Name,
		SourceType: req.SourceType,
		SourceData: req.SourceData,
		SourceURL:  req.SourceURL,
		Enabled:    enabled,
		SortOrder:  req.SortOrder,
	}
	if err := h.sourceRepo.Create(ctx, source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create individual nodes (proxy_json is always normalized mihomo proxy map)
	var nodes []entity.ExternalNode
	for _, pm := range proxyMaps {
		pm = normalizeProxyMap(pm)
		proxyJSON, _ := json.Marshal(pm)
		nodes = append(nodes, entity.ExternalNode{
			SourceID:   source.ID,
			Name:       getString(pm, "name"),
			Protocol:   getString(pm, "type"),
			Host:       getString(pm, "server"),
			Port:       getInt(pm, "port"),
			Transport:  getStringDefault(pm, "network", "tcp"),
			TLS:        getBool(pm, "tls"),
			ServerName: getStringDefault(pm, "servername", getString(pm, "sni")),
			ProxyJSON:  string(proxyJSON),
			Enabled:    true,
		})
	}
	if len(nodes) > 0 {
		h.nodeRepo.CreateBatch(ctx, nodes)
	}

	c.JSON(http.StatusOK, gin.H{"data": source, "node_count": len(nodes)})
}

func (h *ExternalNodeHandler) UpdateSource(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.sourceRepo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.sourceRepo.Update(ctx, existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": existing})
}

func (h *ExternalNodeHandler) DeleteSource(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Delete all nodes from this source first
	h.nodeRepo.DeleteBySourceID(ctx, id)
	h.sourceRepo.Delete(ctx, id)
	c.JSON(http.StatusOK, gin.H{"data": "deleted"})
}

func (h *ExternalNodeHandler) Parse(c *gin.Context) {
	var req struct {
		SourceType string `json:"source_type"`
		SourceData string `json:"source_data"`
		SourceURL  string `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	proxyMaps, err := parseToProxyMaps(req.SourceType, req.SourceData, req.SourceURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": proxyMaps, "count": len(proxyMaps)})
}

// --- Node CRUD ---

func (h *ExternalNodeHandler) ListNodes(c *gin.Context) {
	ctx := c.Request.Context()
	var sourceID int64
	if v := c.Query("source_id"); v != "" {
		sourceID, _ = strconv.ParseInt(v, 10, 64)
	}
	nodes, err := h.nodeRepo.List(ctx, sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (h *ExternalNodeHandler) GetNode(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	node, err := h.nodeRepo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": node})
}

func (h *ExternalNodeHandler) UpdateNode(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.nodeRepo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Enabled   *bool   `json:"enabled"`
		SortOrder *int    `json:"sort_order"`
		ProxyJSON *string `json:"proxy_json"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}
	if req.ProxyJSON != nil {
		existing.ProxyJSON = *req.ProxyJSON
		// Re-parse metadata from proxy JSON
		var pm map[string]interface{}
		if json.Unmarshal([]byte(*req.ProxyJSON), &pm) == nil {
			if v, ok := pm["type"].(string); ok {
				existing.Protocol = v
			}
			if v, ok := pm["server"].(string); ok {
				existing.Host = v
			}
			if v, ok := pm["port"].(string); ok {
				fmt.Sscanf(v, "%d", &existing.Port)
			} else if v, ok := pm["port"].(float64); ok {
				existing.Port = int(v)
			}
			if v, ok := pm["network"].(string); ok {
				existing.Transport = v
			}
			if v, ok := pm["tls"].(bool); ok {
				existing.TLS = v
			}
		}
	}
	if err := h.nodeRepo.Update(ctx, existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": existing})
}

func (h *ExternalNodeHandler) DeleteNode(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.nodeRepo.Delete(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "deleted"})
}

// --- Helpers ---

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getStringDefault(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(string); ok {
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// normalizeProxyMap ensures the proxy map uses standard mihomo field names.
func normalizeProxyMap(pm map[string]interface{}) map[string]interface{} {
	// hysteria → hysteria2
	if t, _ := pm["type"].(string); t == "hysteria" {
		pm["type"] = "hysteria2"
	}
	// raw → tcp
	if n, _ := pm["network"].(string); n == "raw" || n == "" {
		pm["network"] = "tcp"
	}
	// Remove unsupported transports
	if n, _ := pm["network"].(string); n == "kcp" || n == "mkcp" || n == "quic" {
		pm["network"] = "tcp"
	}
	// Ensure name exists
	if _, ok := pm["name"]; !ok {
		pm["name"] = "unnamed"
	}
	return pm
}

// --- Parsing helpers ---

func parseToProxyMaps(sourceType, sourceData, sourceURL string) ([]map[string]interface{}, error) {
	if sourceType == "url" && sourceURL != "" {
		// ⚠ 这是**用户提供的 URL**，必须按不可信输入处理（安全审计 v5 S1）：
		// 旧实现 `http.Get` 无超时、**跟随重定向**、完全不校验内网地址 → 可被用来读云元数据
		// （169.254.169.254）或内网服务，而且抓到的内容会被解析后**回显给调用方**。
		// 另外旧实现只调了一次 `Body.Read` ✗ —— 单次 Read 可能只返回部分内容，
		// 订阅稍大就会被静默截断；改成有上限的完整读取，顺带修掉这个截断问题。
		body, err := netutil.SafeFetchGET(context.Background(), sourceURL, 20*time.Second,
			netutil.DefaultSafeFetchMaxBytes)
		if err != nil {
			return nil, fmt.Errorf("fetch url: %w", err)
		}
		sourceData = strings.TrimSpace(string(body))
		if strings.HasPrefix(sourceData, "{") || strings.HasPrefix(sourceData, "[") {
			sourceType = "json"
		} else if strings.HasPrefix(sourceData, "proxies:") || strings.HasPrefix(sourceData, "- name:") {
			sourceType = "yaml"
		} else {
			sourceType = "uri"
		}
	}

	switch sourceType {
	case "yaml":
		return parseYAMLToProxyMaps(sourceData)
	case "json":
		return parseJSONToProxyMaps(sourceData)
	case "uri":
		return parseURIToProxyMaps(sourceData)
	case "base64_uri":
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sourceData))
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(sourceData))
			if err != nil {
				return nil, fmt.Errorf("base64 decode error: %w", err)
			}
		}
		return parseURIToProxyMaps(string(decoded))
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

func parseURIToProxyMaps(data string) ([]map[string]interface{}, error) {
	return mihomoconv.ConvertsV2Ray([]byte(data))
}

func parseYAMLToProxyMaps(data string) ([]map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &parsed); err != nil {
		// Try parsing as a bare array (just proxies list)
		var arr []map[string]interface{}
		if err2 := yaml.Unmarshal([]byte(data), &arr); err2 == nil {
			return arr, nil
		}
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}

	// Look for "proxies" key
	if proxies, ok := parsed["proxies"].([]interface{}); ok {
		var result []map[string]interface{}
		for _, p := range proxies {
			if m, ok := p.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result, nil
	}

	// Maybe the whole thing is a single proxy
	if _, ok := parsed["name"].(string); ok {
		return []map[string]interface{}{parsed}, nil
	}

	return nil, fmt.Errorf("no proxies found in YAML")
}

func parseJSONToProxyMaps(data string) ([]map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no outbounds found")
	}
	var proxyMaps []map[string]interface{}
	for _, ob := range outbounds {
		obMap, ok := ob.(map[string]interface{})
		if !ok {
			continue
		}
		uri := outboundMapToURI(obMap)
		if uri != "" {
			maps, _ := mihomoconv.ConvertsV2Ray([]byte(uri))
			proxyMaps = append(proxyMaps, maps...)
		}
	}
	return proxyMaps, nil
}

func outboundMapToURI(ob map[string]interface{}) string {
	protocol, _ := ob["protocol"].(string)
	tag, _ := ob["tag"].(string)
	settings, _ := ob["settings"].(map[string]interface{})
	ss, _ := ob["streamSettings"].(map[string]interface{})

	getFirstUser := func(arr []interface{}) map[string]interface{} {
		if len(arr) == 0 {
			return nil
		}
		u, _ := arr[0].(map[string]interface{})
		return u
	}

	switch protocol {
	case "vless":
		vnext, ok := settings["vnext"].([]interface{})
		if !ok || len(vnext) == 0 {
			return ""
		}
		v, _ := vnext[0].(map[string]interface{})
		address, _ := v["address"].(string)
		port := 0
		if p, ok := v["port"].(float64); ok {
			port = int(p)
		}
		users, _ := v["users"].([]interface{})
		user := getFirstUser(users)
		if user == nil {
			return ""
		}
		uuid, _ := user["id"].(string)
		flow, _ := user["flow"].(string)
		uri := fmt.Sprintf("vless://%s@%s:%d", uuid, address, port)
		params := []string{}
		if flow != "" {
			params = append(params, "flow="+flow)
		}
		if enc, ok := settings["encryption"].(string); ok && enc != "" && enc != "none" {
			params = append(params, "encryption="+enc)
		}
		if ss != nil {
			if net, ok := ss["network"].(string); ok && net != "" {
				params = append(params, "type="+net)
			}
			if sec, ok := ss["security"].(string); ok && sec != "none" {
				params = append(params, "security="+sec)
			}
		}
		if len(params) > 0 {
			uri += "?" + strings.Join(params, "&")
		}
		uri += "#" + tag
		return uri

	case "vmess":
		vnext, ok := settings["vnext"].([]interface{})
		if !ok || len(vnext) == 0 {
			return ""
		}
		v, _ := vnext[0].(map[string]interface{})
		address, _ := v["address"].(string)
		port := 0
		if p, ok := v["port"].(float64); ok {
			port = int(p)
		}
		users, _ := v["users"].([]interface{})
		user := getFirstUser(users)
		if user == nil {
			return ""
		}
		uuid, _ := user["id"].(string)
		uri := fmt.Sprintf("vmess://%s@%s:%d", uuid, address, port)
		params := []string{}
		if ss != nil {
			if net, ok := ss["network"].(string); ok && net != "" {
				params = append(params, "type="+net)
			}
		}
		if len(params) > 0 {
			uri += "?" + strings.Join(params, "&")
		}
		uri += "#" + tag
		return uri

	case "trojan":
		servers, ok := settings["servers"].([]interface{})
		if !ok || len(servers) == 0 {
			return ""
		}
		s, _ := servers[0].(map[string]interface{})
		address, _ := s["address"].(string)
		port := 0
		if p, ok := s["port"].(float64); ok {
			port = int(p)
		}
		password, _ := s["password"].(string)
		uri := fmt.Sprintf("trojan://%s@%s:%d", urlEncode(password), address, port)
		uri += "#" + tag
		return uri

	case "shadowsocks":
		servers, ok := settings["servers"].([]interface{})
		if !ok || len(servers) == 0 {
			return ""
		}
		s, _ := servers[0].(map[string]interface{})
		address, _ := s["address"].(string)
		port := 0
		if p, ok := s["port"].(float64); ok {
			port = int(p)
		}
		password, _ := s["password"].(string)
		method, _ := s["method"].(string)
		if method == "" {
			method = "aes-256-gcm"
		}
		// ss://base64(method:password)@host:port#tag
		auth := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
		uri := fmt.Sprintf("ss://%s@%s:%d#%s", auth, address, port, tag)
		return uri
	}
	return ""
}

func urlEncode(s string) string {
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, "=", "%3D")
	return s
}
