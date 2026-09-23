// Package exportutil 提供 CSV 导出共用的三件事：**可配置的行数上限**、稳定的字段转义、Excel 兼容 BOM。
//
// 为什么要有这个包 ✗：这三件事原来在各导出 handler 里**各写一遍** ——
// 行数上限在审计导出写死 5 万、在充值卡导出写死 10 万（想调只能改代码重编译 ✗），
// 转义与 BOM 也各自实现（容易有的加 BOM、有的忘 ✓）。
// 现在统一走 `panel.export_max_rows` 设置项（设置页可调 ✓）+ 同一套转义 ✓。
package exportutil

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/service/system"
)

// BOM 是 UTF-8 BOM ✗：不加的话 Excel 打开中文列（备注、错误信息等）会乱码 ✓。
const BOM = "\xEF\xBB\xBF"

// MaxRows 读取可配置的导出上限（`system_settings.panel.export_max_rows`）✓。
//
// 读不到、值为空或非法时一律回落到默认值 ✓ —— 导出不该因为设置缺失就失败 ✗。
func MaxRows(ctx context.Context, db *gorm.DB) int {
	if db != nil {
		var value string
		err := db.WithContext(ctx).
			Table("system_settings").
			Select("value").
			Where("key = ?", "panel.export_max_rows").
			Scan(&value).Error
		if err == nil && strings.TrimSpace(value) != "" {
			if n, convErr := strconv.Atoi(strings.TrimSpace(value)); convErr == nil {
				return system.NormalizeExportMaxRows(n)
			}
		}
	}
	return system.DefaultExportMaxRows
}

// Limit 计算本次导出实际取多少行：尊重前端传的 `?limit=`（通常想少取一些 ✓），
// 但**绝不超过**配置上限 ✓（防止一次导出把内存/响应体撑爆 ✗）。
func Limit(c *gin.Context, max int) int {
	if max <= 0 {
		max = system.DefaultExportMaxRows
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < max {
			return n
		}
	}
	return max
}

// Escape 按 RFC 4180 转义：字段含逗号 / 引号 / 换行时整体加双引号，并把内部 `"` 变成 `""` ✓。
//
// ⚠ 别省这一步 ✗：备注、错误信息、元数据里经常带逗号和引号，
// 不转义会让导出的 CSV **列错位**（Excel 里看起来就是数据串行 ✓）。
func Escape(s string) string {
	if !strings.ContainsAny(s, ",\"\n\r") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// Timestamp 返回用于文件名的紧凑时间串（导出文件名统一带上时间 ✗ 避免多次导出互相覆盖 ✓）。
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}
