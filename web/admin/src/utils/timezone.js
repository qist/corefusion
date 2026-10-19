/**
 * 常用 IANA 时区列表
 *
 * 初始化向导与「系统设置 → 面板」共用同一份列表，避免两处选项不一致。
 * 值必须是 tzdata 能解析的名称：后端会写入 panel.timezone，并注入代理进程的 TZ。
 */
export const TIMEZONES = [
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Taipei',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Bangkok',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'Europe/Moscow',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Australia/Sydney',
  'Pacific/Auckland',
  'UTC'
]
