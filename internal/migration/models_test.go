package migration

import (
	"fmt"
	"testing"
)

// TestModelsUnique 防止模型清单再次漂移：
// 1) 不能有重复模型（重复 AutoMigrate 会做无用功）；
// 2) 表名必须唯一；
// 3) 数量守住 —— 目前 118 个模型，少一个就说明又漏表了。
func TestModelsUnique(t *testing.T) {
	models := Models()
	if len(models) != 118 {
		t.Fatalf("Models() has %d models, want 118 (update this test when adding a model intentionally)", len(models))
	}

	seenType := make(map[string]struct{}, len(models))
	seenTable := make(map[string]struct{}, len(models))
	for _, m := range models {
		if m == nil {
			t.Fatal("Models() contains nil model")
		}
		typeName := fmt.Sprintf("%T", m)
		if _, ok := seenType[typeName]; ok {
			t.Fatalf("duplicate model type %q in Models()", typeName)
		}
		seenType[typeName] = struct{}{}

		if namer, ok := m.(interface{ TableName() string }); ok {
			tn := namer.TableName()
			if _, dup := seenTable[tn]; dup {
				t.Fatalf("duplicate table %q in Models()", tn)
			}
			seenTable[tn] = struct{}{}
		}
	}
}
