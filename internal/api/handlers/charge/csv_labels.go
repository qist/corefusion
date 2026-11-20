package charge

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// csvLabels 是「导出 CSV」的表头与枚举文案。
//
// 为什么需要它：CSV 是直接给人看的文件，硬编码中文会让英文/日文后台用户
// 导出后完全读不懂。这里按 ?lang= 选择语言，未识别的语言一律回落英文
// （宁可英文，也不要把中文塞给非中文用户）。
type csvLabels struct {
	CardHeader               []string
	CardTypeBalance          string
	CardTypePlan             string
	CardTypeTrafficPack      string
	Yes                      string
	No                       string
	OrderHeader              []string
	BalanceTransactionHeader []string
}

// csvLang 归一导出语言：zh-CN / zh-TW / en（其余一律 en）。
func csvLang(c *gin.Context) string {
	l := strings.ToLower(strings.TrimSpace(c.Query("lang")))
	switch {
	case strings.HasPrefix(l, "zh-tw"), strings.HasPrefix(l, "zh-hk"),
		strings.HasPrefix(l, "zh-hant"), strings.HasPrefix(l, "zh_hk"):
		return "zh-TW"
	case strings.HasPrefix(l, "zh"):
		return "zh-CN"
	default:
		return "en"
	}
}

func csvLabelsFor(lang string) csvLabels {
	switch lang {
	case "zh-CN":
		return csvLabels{
			CardHeader: []string{
				"ID", "卡号", "类型", "面值/套餐ID", "货币", "是否已用",
				"状态", "批次号", "创建者", "创建时间", "过期时间", "备注",
			},
			CardTypeBalance:     "金额",
			CardTypePlan:        "套餐",
			CardTypeTrafficPack: "流量包",
			Yes:                 "是",
			No:                  "否",
			OrderHeader: []string{
				"ID", "交易号", "站点ID", "站点交易号", "客户端ID", "客户端邮箱",
				"订单类型", "金额(最小单位)", "币种", "实付金额(最小单位)", "实付币种",
				"汇率", "支付方式", "状态", "创建时间", "支付时间", "备注",
			},
			BalanceTransactionHeader: []string{
				"ID", "客户端ID", "客户端邮箱", "类型", "金额(最小单位)",
				"变动前(最小单位)", "变动后(最小单位)", "币种",
				"关联充值订单ID", "关联套餐订单ID", "备注", "时间",
			},
		}
	case "zh-TW":
		return csvLabels{
			CardHeader: []string{
				"ID", "卡號", "類型", "面額/方案ID", "貨幣", "是否已用",
				"狀態", "批次號", "建立者", "建立時間", "到期時間", "備註",
			},
			CardTypeBalance:     "金額",
			CardTypePlan:        "方案",
			CardTypeTrafficPack: "流量包",
			Yes:                 "是",
			No:                  "否",
			OrderHeader: []string{
				"ID", "交易號", "站點ID", "站點交易號", "用戶端ID", "用戶端信箱",
				"訂單類型", "金額(最小單位)", "幣種", "實付金額(最小單位)", "實付幣種",
				"匯率", "付款方式", "狀態", "建立時間", "付款時間", "備註",
			},
			BalanceTransactionHeader: []string{
				"ID", "用戶端ID", "用戶端信箱", "類型", "金額(最小單位)",
				"變動前(最小單位)", "變動後(最小單位)", "幣種",
				"關聯充值訂單ID", "關聯方案訂單ID", "備註", "時間",
			},
		}
	default:
		return csvLabels{
			CardHeader: []string{
				"ID", "Card Code", "Type", "Face Value / Plan ID", "Currency", "Used",
				"Status", "Batch ID", "Created By", "Created At", "Expires At", "Remark",
			},
			CardTypeBalance:     "Balance",
			CardTypePlan:        "Plan",
			CardTypeTrafficPack: "Traffic Pack",
			Yes:                 "Yes",
			No:                  "No",
			OrderHeader: []string{
				"ID", "Trade No", "Site ID", "Site Trade No", "Client ID", "Client Email",
				"Order Type", "Amount (minor unit)", "Currency", "Paid Amount (minor unit)",
				"Paid Currency", "Exchange Rate", "Payment Method", "Status",
				"Created At", "Paid At", "Remark",
			},
			BalanceTransactionHeader: []string{
				"ID", "Client ID", "Client Email", "Type", "Amount (minor unit)",
				"Balance Before (minor unit)", "Balance After (minor unit)", "Currency",
				"Related Charge Order ID", "Related Plan Order ID", "Remark", "Time",
			},
		}
	}
}
