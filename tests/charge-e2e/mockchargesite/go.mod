// 独立测试模块 ✓：模拟充值站点只在测试时用，不参与产品构建 ✗。
// 放在独立 go.mod 里 → 主模块的 `go build ./...` 不会包含它 ✓（零污染 ✓）。
module mockchargesite

go 1.21
