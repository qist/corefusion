// statsprobe 直接查询核心的 stats gRPC 接口并打印所有计数器，
// 用于排查面板流量采集问题（xray / sing-box 的 stats protobuf 一致但 gRPC 服务名不同，
// 用错服务名会得到 Unimplemented，表现为「采集静默失效」）。
//
// 用法：
//
//	go run ./cmd/statsprobe                                  # 默认 sing-box @127.0.0.1:10086（不重置）
//	go run ./cmd/statsprobe 127.0.0.1:10086 reset singbox     # 取增量并重置计数器
//	go run ./cmd/statsprobe 127.0.0.1:10085 nostore xray      # 查 xray（不重置）
//
// 参数：<地址> <reset|nostore> <singbox|xray>
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"corefusion/internal/coredata"
)

func main() {
	addr := "127.0.0.1:10086"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	reset := len(os.Args) > 2 && os.Args[2] == "reset"
	core := "singbox"
	if len(os.Args) > 3 {
		core = os.Args[3]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// xray 与 sing-box 的 stats protobuf 一致但 gRPC 服务名不同，必须按核心选择。
	var client *coredata.XrayClient
	if core == "xray" {
		client = coredata.NewXrayClient(addr)
	} else {
		client = coredata.NewV2RayClient(addr)
	}
	resp, err := client.RawStats(ctx, reset)
	if err != nil {
		fmt.Printf("RawStats 失败 (%s): %v\n", addr, err)
		os.Exit(1)
	}
	fmt.Printf("地址 %s | reset=%v | 计数器数量=%d\n\n", addr, reset, len(resp.GetStat()))

	names := make([]string, 0, len(resp.GetStat()))
	byName := map[string]int64{}
	for _, s := range resp.GetStat() {
		names = append(names, s.GetName())
		byName[s.GetName()] = s.GetValue()
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("  %-58s %d\n", n, byName[n])
	}

	if sr, err := coredata.ParseStatsResponse(resp); err == nil {
		fmt.Printf("\n解析结果: 用户=%d 入站=%d\n", len(sr.Users), len(sr.Inbounds))
		for _, u := range sr.Users {
			fmt.Printf("  user %-30s up=%-12d down=%-12d total=%d\n", u.Email, u.Uplink, u.Downlink, u.Total)
		}
	} else {
		fmt.Println("解析失败:", err)
	}
}
