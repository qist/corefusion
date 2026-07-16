package constants

// CurrencyInfo describes a supported currency.
type CurrencyInfo struct {
	Code     string // ISO 4217 货币代码
	Name     string // 货币名称
	NameCN   string // 中文名称
	Symbol   string // 货币符号
	Decimals int    // 最小单位的小数位数 (如 CNY 的分=2, IRR 无小数=0)
	UnitName string // 最小单位名称 (如 分, 戈比, 等)
}

// SupportedCurrencies is the full list of supported currencies.
// Decimals: 后端存储使用最小单位（如 CNY 存分=元×100, USD 存分=元×100, IRR 存里亚尔无换算）。
var SupportedCurrencies = []CurrencyInfo{
	{Code: "CNY", Name: "Chinese Yuan", NameCN: "人民币", Symbol: "¥", Decimals: 2, UnitName: "分"},
	{Code: "USD", Name: "US Dollar", NameCN: "美元", Symbol: "$", Decimals: 2, UnitName: "cent"},
	{Code: "EUR", Name: "Euro", NameCN: "欧元", Symbol: "€", Decimals: 2, UnitName: "cent"},
	{Code: "GBP", Name: "British Pound", NameCN: "英镑", Symbol: "£", Decimals: 2, UnitName: "penny"},
	{Code: "RUB", Name: "Russian Ruble", NameCN: "俄罗斯卢布", Symbol: "₽", Decimals: 2, UnitName: "戈比"},
	{Code: "IRR", Name: "Iranian Rial", NameCN: "伊朗里亚尔", Symbol: "﷼", Decimals: 0, UnitName: "里亚尔"},
	{Code: "AED", Name: "UAE Dirham", NameCN: "阿联酋迪拉姆", Symbol: "د.إ", Decimals: 2, UnitName: "fils"},
	{Code: "TRY", Name: "Turkish Lira", NameCN: "土耳其里拉", Symbol: "₺", Decimals: 2, UnitName: "kuruş"},
	{Code: "JPY", Name: "Japanese Yen", NameCN: "日元", Symbol: "¥", Decimals: 0, UnitName: "円"},
	{Code: "KRW", Name: "South Korean Won", NameCN: "韩元", Symbol: "₩", Decimals: 0, UnitName: "원"},
	{Code: "INR", Name: "Indian Rupee", NameCN: "印度卢比", Symbol: "₹", Decimals: 2, UnitName: "paisa"},
	{Code: "BRL", Name: "Brazilian Real", NameCN: "巴西雷亚尔", Symbol: "R$", Decimals: 2, UnitName: "centavo"},
	{Code: "AUD", Name: "Australian Dollar", NameCN: "澳元", Symbol: "A$", Decimals: 2, UnitName: "cent"},
	{Code: "CAD", Name: "Canadian Dollar", NameCN: "加元", Symbol: "C$", Decimals: 2, UnitName: "cent"},
	{Code: "CHF", Name: "Swiss Franc", NameCN: "瑞士法郎", Symbol: "Fr", Decimals: 2, UnitName: "rappen"},
	{Code: "HKD", Name: "Hong Kong Dollar", NameCN: "港币", Symbol: "HK$", Decimals: 2, UnitName: "cent"},
	{Code: "SGD", Name: "Singapore Dollar", NameCN: "新加坡元", Symbol: "S$", Decimals: 2, UnitName: "cent"},
	{Code: "MYR", Name: "Malaysian Ringgit", NameCN: "马来西亚林吉特", Symbol: "RM", Decimals: 2, UnitName: "sen"},
	{Code: "THB", Name: "Thai Baht", NameCN: "泰铢", Symbol: "฿", Decimals: 2, UnitName: "satang"},
	{Code: "VND", Name: "Vietnamese Dong", NameCN: "越南盾", Symbol: "₫", Decimals: 0, UnitName: "hào"},
}

// CurrencyMap is a lookup map by currency code.
var CurrencyMap = make(map[string]CurrencyInfo)

func init() {
	for _, c := range SupportedCurrencies {
		CurrencyMap[c.Code] = c
	}
}

// GetCurrencyDecimals returns the number of decimal places for a currency.
// Returns 2 as default if not found.
func GetCurrencyDecimals(code string) int {
	if c, ok := CurrencyMap[code]; ok {
		return c.Decimals
	}
	return 2
}

// GetCurrencySymbol returns the symbol for a currency.
// Returns the code itself if not found.
func GetCurrencySymbol(code string) string {
	if c, ok := CurrencyMap[code]; ok {
		return c.Symbol
	}
	return code
}

// IsCurrencySupported checks if a currency code is supported.
func IsCurrencySupported(code string) bool {
	_, ok := CurrencyMap[code]
	return ok
}
