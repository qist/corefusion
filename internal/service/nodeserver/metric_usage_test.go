package nodeserver

import (
	"testing"

	"corefusion/internal/model/entity"
)

// 历史库里存在负速率（差分回退）；读接口必须裁剪成 0，
// 不能把负值透给前端概览/图表。
func TestMetricToResourceUsageClampsNegativeRates(t *testing.T) {
	got := metricToResourceUsage(&entity.ServerMetric{
		ID:              1,
		NetRxBytes:      -10,
		TrafficUplink:   -34455,
		TrafficDownlink: -160668,
	})
	if got == nil {
		t.Fatal("usage should not be nil")
	}
	if got.NetRxBytesSec != 0 || got.TrafficUplink != 0 || got.TrafficDownlink != 0 {
		t.Fatalf("negative rates should be clamped, got %+v", got)
	}

	ok := metricToResourceUsage(&entity.ServerMetric{ID: 2, NetRxBytes: 306, TrafficUplink: 1234})
	if ok.NetRxBytesSec != 306 || ok.TrafficUplink != 1234 {
		t.Fatalf("positive rates should be kept, got %+v", ok)
	}
}
