package coredata

import (
	"testing"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

func TestParseStatsResponse(t *testing.T) {
	resp := &statsService.QueryStatsResponse{
		Stat: []*statsService.Stat{
			{Name: "user>>>alice@example.com>>>traffic>>>uplink", Value: 100},
			{Name: "user>>>alice@example.com>>>traffic>>>downlink", Value: 240},
			{Name: "user>>>bob@example.com>>>traffic>>>uplink", Value: 10},
			{Name: "user>>>bob@example.com>>>traffic>>>downlink", Value: 20},
			{Name: "inbound>>>grpc>>>traffic>>>uplink", Value: 5},
			{Name: "inbound>>>grpc>>>traffic>>>downlink", Value: 7},
			{Name: "outbound>>>freedom>>>traffic>>>uplink", Value: 1}, // 非 inbound>>> 前缀，忽略
		},
	}

	sr, err := ParseStatsResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(sr.Users))
	}
	if len(sr.Inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(sr.Inbounds))
	}

	// 校验 alice 累计值
	var alice *UserTraffic
	for i := range sr.Users {
		if sr.Users[i].Email == "alice@example.com" {
			alice = &sr.Users[i]
		}
	}
	if alice == nil {
		t.Fatal("alice not found")
	}
	if alice.Uplink != 100 || alice.Downlink != 240 || alice.Total != 340 {
		t.Fatalf("alice traffic wrong: %+v", alice)
	}

	if sr.Inbounds[0].Tag != "grpc" || sr.Inbounds[0].Uplink != 5 || sr.Inbounds[0].Downlink != 7 || sr.Inbounds[0].Total != 12 {
		t.Fatalf("inbound traffic wrong: %+v", sr.Inbounds[0])
	}
}
