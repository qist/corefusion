package agent

import "testing"

// 计数器回退（核心重启 / trafficsync reset_=true 抢读）会让差分为负，
// 落库前必须裁剪为 0，否则实时图表会画出负速率。
func TestNonNegativeRateClampsCounterReset(t *testing.T) {
	if got := nonNegativeRate(-160668); got != 0 {
		t.Fatalf("negative rate should clamp to 0, got %d", got)
	}
	if got := nonNegativeRate(34455); got != 34455 {
		t.Fatalf("positive rate should be kept, got %d", got)
	}
}
