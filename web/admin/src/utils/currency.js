/**
 * 国际化货币映射表
 * 
 * 每种货币包含：
 * - code: ISO 4217 货币代码
 * - symbol: 货币符号
 * - decimals: 最小单位小数位数 (CNY/USD=2, JPY/IRR=0)
 * - unitName: 最小单位名称 (分/cent/戈比等)
 * 
 * 金额转换：
 * - decimals=2 的货币: 元 → 分 (value = yuan × 100)
 * - decimals=0 的货币: 无转换 (value = 面值本身)
 */

export const CURRENCIES = [
  { code: 'CNY', symbol: '¥', decimals: 2, unitName: '分',   unitNameEn: 'cent',   nameKey: 'currency.CNY' },
  { code: 'USD', symbol: '$', decimals: 2, unitName: '分',   unitNameEn: 'cent',   nameKey: 'currency.USD' },
  { code: 'EUR', symbol: '€', decimals: 2, unitName: '分',   unitNameEn: 'cent',   nameKey: 'currency.EUR' },
  { code: 'GBP', symbol: '£', decimals: 2, unitName: '便士', unitNameEn: 'penny',  nameKey: 'currency.GBP' },
  { code: 'RUB', symbol: '₽', decimals: 2, unitName: '戈比', unitNameEn: 'kopek',  nameKey: 'currency.RUB' },
  { code: 'IRR', symbol: '﷼', decimals: 0, unitName: '里亚尔', unitNameEn: 'rial', nameKey: 'currency.IRR' },
  { code: 'AED', symbol: 'د.إ', decimals: 2, unitName: '费尔', unitNameEn: 'fils', nameKey: 'currency.AED' },
  { code: 'TRY', symbol: '₺', decimals: 2, unitName: '库鲁', unitNameEn: 'kuruş', nameKey: 'currency.TRY' },
  { code: 'JPY', symbol: '¥', decimals: 0, unitName: '円',   unitNameEn: 'yen',    nameKey: 'currency.JPY' },
  { code: 'KRW', symbol: '₩', decimals: 0, unitName: '원',   unitNameEn: 'won',    nameKey: 'currency.KRW' },
  { code: 'INR', symbol: '₹', decimals: 2, unitName: '派萨', unitNameEn: 'paisa', nameKey: 'currency.INR' },
  { code: 'BRL', symbol: 'R$', decimals: 2, unitName: '分', unitNameEn: 'centavo', nameKey: 'currency.BRL' },
  { code: 'AUD', symbol: 'A$', decimals: 2, unitName: '分', unitNameEn: 'cent',    nameKey: 'currency.AUD' },
  { code: 'CAD', symbol: 'C$', decimals: 2, unitName: '分', unitNameEn: 'cent',    nameKey: 'currency.CAD' },
  { code: 'CHF', symbol: 'Fr', decimals: 2, unitName: '分', unitNameEn: 'rappen', nameKey: 'currency.CHF' },
  { code: 'HKD', symbol: 'HK$', decimals: 2, unitName: '分', unitNameEn: 'cent',  nameKey: 'currency.HKD' },
  { code: 'SGD', symbol: 'S$', decimals: 2, unitName: '分', unitNameEn: 'cent',   nameKey: 'currency.SGD' },
  { code: 'MYR', symbol: 'RM', decimals: 2, unitName: '仙', unitNameEn: 'sen',    nameKey: 'currency.MYR' },
  { code: 'THB', symbol: '฿', decimals: 2, unitName: '萨当', unitNameEn: 'satang', nameKey: 'currency.THB' },
  { code: 'VND', symbol: '₫', decimals: 0, unitName: '毫', unitNameEn: 'hào',    nameKey: 'currency.VND' },
]

// 货币代码 → 信息映射
export const CURRENCY_MAP = {}
CURRENCIES.forEach(c => { CURRENCY_MAP[c.code] = c })

/**
 * 获取货币信息，未找到返回默认 CNY
 */
export function getCurrencyInfo(code) {
  return CURRENCY_MAP[code] || CURRENCY_MAP['CNY']
}

/**
 * 获取货币符号
 */
export function getCurrencySymbol(code) {
  return getCurrencyInfo(code).symbol
}

/**
 * 获取货币小数位数
 */
export function getCurrencyDecimals(code) {
  return getCurrencyInfo(code).decimals
}

/**
 * 将元（主单位）转为最小单位（分/戈比等）
 * decimals=2: yuan * 100
 * decimals=0: yuan 本身
 */
export function yuanToMinor(yuan, currencyCode) {
  const info = getCurrencyInfo(currencyCode)
  const factor = Math.pow(10, info.decimals)
  return Math.round(yuan * factor)
}

/**
 * 将最小单位转为元（主单位）
 * decimals=2: minor / 100
 * decimals=0: minor 本身
 */
export function minorToYuan(minor, currencyCode) {
  const info = getCurrencyInfo(currencyCode)
  const factor = Math.pow(10, info.decimals)
  return minor / factor
}

/**
 * 格式化金额显示
 * @param {number} minor 最小单位金额
 * @param {string} currencyCode 货币代码
 * @returns {string} 如 "¥10.50" 或 "₽100,00"
 */
export function formatAmount(minor, currencyCode) {
  const info = getCurrencyInfo(currencyCode)
  const yuan = minorToYuan(minor, currencyCode)
  const formatted = info.decimals > 0
    ? yuan.toFixed(info.decimals)
    : yuan.toFixed(0)
  return `${info.symbol}${formatted}`
}

/**
 * 获取货币选择器的 label
 * 如 "CNY - ¥ (人民币)"
 */
export function getCurrencyLabel(code, locale = 'zh-CN') {
  const info = getCurrencyInfo(code)
  const name = locale === 'en' ? code : info.unitName
  const nativeName = locale === 'en' 
    ? info.unitNameEn 
    : info.unitName
  return `${code} ${info.symbol} (${nativeName})`
}

/**
 * 获取步进值（元输入框的 step）
 * decimals=2 → 0.01
 * decimals=0 → 1
 */
export function getAmountStep(currencyCode) {
  const info = getCurrencyInfo(currencyCode)
  return info.decimals > 0 ? 1 / Math.pow(10, info.decimals) : 1
}

/**
 * 获取面值输入提示
 */
export function getValuePlaceholder(currencyCode) {
  const info = getCurrencyInfo(currencyCode)
  if (info.decimals === 2) return '0.00'
  return '0'
}
