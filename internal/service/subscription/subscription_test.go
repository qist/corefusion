package subscription

import (
	"reflect"
	"testing"

	"corefusion/internal/model/entity"
)

func TestUniquePositiveInt64sPreservesExplicitOrder(t *testing.T) {
	got := uniquePositiveInt64s([]int64{0, 9, 9, -1, 3, 9, 5, 3})
	want := []int64{9, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestFilterAllowedBillingNodeIDsKeepsOnlyEnabledPlanNodes(t *testing.T) {
	got := filterAllowedBillingNodeIDs(
		[]int64{22, 11, 33, 44},
		[]entity.PlanNode{
			{ID: 11, Enabled: true},
			{ID: 22, Enabled: true},
			{ID: 33, Enabled: false},
			{ID: 55, Enabled: true},
		},
	)
	want := entity.Int64List{22, 11}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
