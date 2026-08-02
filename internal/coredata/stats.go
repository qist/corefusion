package coredata

import (
	"strings"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// ParseStatsResponse 将 xray QueryStats 原始响应解析为归一化的用户/入站流量。
//
// 此函数是此前散落在 4 处的解析逻辑（xraystats.handler.queryStats、
// agent.xrayapi.buildStatsResponse、trafficsync.service.sync、
// cmd/agent.trafficCollector.collect）的唯一实现。
//
// 约定（累计语义，reset_=false）：
//   - user>>>email>>>traffic>>>uplink|downlink
//   - inbound>>>tag>>>traffic>>>uplink|downlink
//
// 调用方若为增量采集（reset_=true），应在此之后自行按 value<=0 / 跳过首样本
// 等策略过滤，保持原流量上报链路行为不变。
func ParseStatsResponse(resp *statsService.QueryStatsResponse) (*StatsResponse, error) {
	users := make(map[string]*UserTraffic)
	inbounds := make(map[string]*InboundTraffic)

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		if strings.Contains(name, "user>>>") && strings.Contains(name, ">>>traffic>>>") {
			parts := strings.Split(name, ">>>")
			if len(parts) == 4 {
				email := parts[1]
				direction := parts[3]
				if users[email] == nil {
					users[email] = &UserTraffic{Email: email}
				}
				switch direction {
				case DirUplink:
					users[email].Uplink = value
				case DirDownlink:
					users[email].Downlink = value
				}
			}
			continue
		}

		if strings.Contains(name, "inbound>>>") && strings.Contains(name, ">>>traffic>>>") {
			parts := strings.Split(name, ">>>")
			if len(parts) == 4 {
				tag := parts[1]
				direction := parts[3]
				if inbounds[tag] == nil {
					inbounds[tag] = &InboundTraffic{Tag: tag}
				}
				switch direction {
				case DirUplink:
					inbounds[tag].Uplink = value
				case DirDownlink:
					inbounds[tag].Downlink = value
				}
			}
		}
	}

	userList := make([]UserTraffic, 0, len(users))
	for _, u := range users {
		u.Total = u.Uplink + u.Downlink
		userList = append(userList, *u)
	}
	inboundList := make([]InboundTraffic, 0, len(inbounds))
	for _, ib := range inbounds {
		ib.Total = ib.Uplink + ib.Downlink
		inboundList = append(inboundList, *ib)
	}

	return &StatsResponse{Users: userList, Inbounds: inboundList}, nil
}
