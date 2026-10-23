<!-- source: notification-templates.md | locale: zh-TW | updated: 2026-10-23 -->
# 通知模板變數說明

> 語言： [简体中文](notification-templates.md) · [English](notification-templates.en.md) · 繁體中文 · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

通知模板使用 `{變數名}` 佔位符，傳送時會被替換為實際值。本文件列出所有支援的變數、各事件的可用變數，以及自動觸發規則。

## 一、模板型別與受眾

| 受眾 | 渠道 | 收件人 | 典型用途 |
|---|---|---|---|
| **面板使用者**（admin） | Telegram / Email | 管理員配置的渠道與收件人 | 節點告警、備份報告、系統公告 |
| **訂閱者**（subscriber） | Email | 客戶本人郵箱（系統自動按客戶填充） | 餘額不足、訂閱到期提醒 |

## 二、通用變數

所有模板均可使用：

| 變數 | 說明 | 示例值 |
|---|---|---|
| `{time}` | 傳送時間 | `2026-09-28 14:30:00` |
| `{user}` | 客戶郵箱（訂閱者模板）/ 使用者標識 | `user@example.com` |
| `{message}` | 通知正文內容 | 系統維護公告正文 |

## 三、訂閱者事件變數

### 訂閱到期提醒（event_key = `subscription_expire`）

每日自動掃描，發給 7 天內到期的有效訂閱客戶：

| 變數 | 說明 | 示例值 |
|---|---|---|
| `{user}` | 客戶郵箱 | `user@example.com` |
| `{expire_date}` | 訂閱到期時間 | `2026-10-05 09:00` |
| `{days}` | 剩餘天數 | `7` |
| `{time}` | 傳送時間 | `2026-09-28 09:05:00` |

### 餘額不足提醒（event_key = `balance_low`）

每日自動掃描，發給餘額低於閾值的客戶：

| 變數 | 說明 | 示例值 |
|---|---|---|
| `{user}` | 客戶郵箱 | `user@example.com` |
| `{balance}` | 當前餘額（元，自動保留兩位小數） | `0.85` |
| `{time}` | 傳送時間 | `2026-09-28 09:05:00` |

## 四、運維模板示例變數

面板使用者模板在點選「傳送」做連通性測試時，會用以下示例值填充佔位符：

| 變數 | 說明 | 示例值 |
|---|---|---|
| `{node}` | 節點名稱 | `node-01` |
| `{backup_file}` | 備份檔名 | `corefusion-20260101-0900.tar.gz` |
| `{size}` | 檔案大小 | `128MB` |
| `{domain}` | 域名 | `example.com` |
| `{expire_date}` | 到期日期 | `2026-10-12` |
| `{days}` | 剩餘天數 | `14` |
| `{count}` | 數量 | `3` |

## 五、自動觸發規則

- **觸發時間**：面板啟動 30 秒後執行一次首次掃描，之後每天 **09:05** 執行
- **訂閱到期**：掃描 7 天內到期的有效訂閱（`status=active`），客戶狀態也需為有效
- **餘額不足**：掃描餘額 ≤ 閾值的有效客戶；閾值透過系統設定 `notify.balance_threshold` 配置，**單位為分**（預設 `100` = 1.00 元）
- **防重複**：同一客戶、同一事件**每天最多傳送一條**（`notification_send_logs` 按天去重）

## 六、節點健康監控（監控類通知）

按頻率持續檢查節點心跳，**狀態發生變化才發通知，狀態未變化的輪次全部靜默跳過**：

| 設定鍵 | 說明 | 預設值 |
|---|---|---|
| `monitor.node_interval_seconds` | 監控頻率（秒） | `60`（最小 30） |
| `monitor.node_offline_timeout_seconds` | 心跳超時判定閾值（秒） | `120` |

- **離線告警**：節點從線上變為離線時，傳送 admin 模板「**節點離線告警**」，變數 `{node}`（節點名）、`{time}`
- **恢復通知**：節點從離線恢復線上時，傳送 admin 模板「**節點恢復通知**」，變數同上
- **靜默跳過**：狀態未變化的輪次不發任何訊息；面板重啟後首輪只建立基線，不誤報
- **只監控不發**：把對應模板停用（Enabled=OFF）即可

## 七、節點月流量配額告警（自動）

- **觸發**：節點月流量達到配額閾值時發「**節點流量配額告警**」；超過 100% 觸發掛起時再發一條超限告警（事件鍵 `traffic_quota_threshold` / `traffic_quota_exceeded`）
- **閾值**：設定鍵 `monitor.traffic_quota_threshold_percent`（百分比，預設 `80`，設 `0` 關閉閾值告警）
- **去重**：每賬期每節點最多一條（賬期按節點的流量重置日 `traffic_quota_reset_day` 計算）
- 可用變數：`{node}`（節點名）、`{used}`（已用 GB）、`{quota}`（配額 GB）、`{percent}`（百分比）、`{time}`

## 八、備份結果通知（自動）

- **觸發**：定時備份（BackupScheduler）執行後即時傳送——成功發 admin 模板「**每日備份報告**」，失敗（建立備份/讀取儲存/上傳遠端）發「**備份失敗告警**」
- 可用變數：`{backup_file}`（備份名）、`{size}`（大小，僅成功報告）、`{message}`（失敗原因，僅失敗告警）、`{time}`
- **只監控不發**：模板停用（Enabled=OFF）時靜默跳過

## 九、證書到期提醒（自動）

- **觸發**：每日 09:05 掃描即將到期的 `active` 證書（**IP 證書剩 3 天**、**域名證書剩 7 天**），逐張發 admin 模板「**證書到期提醒**」
- **去重**：同一證書每天最多提醒一次；續期後到期日推出對應視窗（IP 3 天 / 域名 7 天）即自動停止
- 可用變數：`{domain}`（證書域名）、`{expire_date}`（到期日期）、`{days}`（剩餘天數）、`{time}`

## 十、核心程序異常告警（自動）

- **觸發**：核心程序（xray/singbox）異常退出或代理程序（caddy/nginx）崩潰時**即時**傳送 admin 模板「**核心程序異常告警**」，守護程序隨後按指數退避自動重啟
- 可用變數：`{core}`（程序型別）、`{restarts}` / `{max}`（重啟次數/上限）、`{message}`（退出資訊，僅核心程序）、`{time}`
- **只監控不發**：模板停用（Enabled=OFF）時靜默跳過

## 十一、新使用者註冊通知（自動）

- **觸發**：客戶註冊成功後**即時**傳送 admin 模板「**新使用者註冊通知**」
- 可用變數：`{user}`（客戶郵箱）、`{time}`

## 十二、使用方式

1. **新增模板**：通知設定 → 通知模板 → 新增模板 → 「從預設填充」選擇預設後按需修改
2. **啟用**：模板需勾選 Enabled 並儲存；自動觸發只處理啟用的模板
3. **測試**：列表中點「傳送」用示例變數實際傳送一條（訂閱者模板的演示郵件發到管理員發件人郵箱）
4. **即時通知**（業務程式碼）：呼叫 `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` 可在事件發生點（充值、扣費、續費）即時發給單個客戶，不受每日掃描限制
