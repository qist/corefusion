<!-- source: notification-templates.md | locale: en | updated: 2026-10-23 -->
# Notification Template Variables

> Languages: [简体中文](notification-templates.md) · English · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Notification templates use `{variable}` placeholders that are replaced with actual values when sending. This document lists all supported variables, the variables available per event, and the automatic trigger rules.

## 1. Template types and audiences

| Audience | Channel | Recipient | Typical use |
|---|---|---|---|
| **Panel users** (admin) | Telegram / Email | Channels and recipients configured by the administrator | Node alerts, backup reports, system announcements |
| **Subscribers** | Email | The customer's own email (filled in automatically) | Low balance, subscription expiry reminders |

## 2. Common variables

Available in all templates:

| Variable | Description | Example |
|---|---|---|
| `{time}` | Send time | `2026-09-28 14:30:00` |
| `{user}` | Customer email (subscriber templates) / user identifier | `user@example.com` |
| `{message}` | Notification body | Maintenance announcement text |

## 3. Subscriber event variables

### Subscription expiry reminder (event_key = `subscription_expire`)

Scanned daily and sent to active subscribers expiring within 7 days:

| Variable | Description | Example |
|---|---|---|
| `{user}` | Customer email | `user@example.com` |
| `{expire_date}` | Subscription expiry | `2026-10-05 09:00` |
| `{days}` | Days remaining | `7` |
| `{time}` | Send time | `2026-09-28 09:05:00` |

### Low balance reminder (event_key = `balance_low`)

Scanned daily and sent to customers whose balance is below the threshold:

| Variable | Description | Example |
|---|---|---|
| `{user}` | Customer email | `user@example.com` |
| `{balance}` | Current balance (currency units, two decimals) | `0.85` |
| `{time}` | Send time | `2026-09-28 09:05:00` |

## 4. Operations template example variables

When you click “Send” for a connectivity test, panel-user templates fill placeholders with these examples:

| Variable | Description | Example |
|---|---|---|
| `{node}` | Node name | `node-01` |
| `{backup_file}` | Backup file name | `corefusion-20260101-0900.tar.gz` |
| `{size}` | File size | `128MB` |
| `{domain}` | Domain | `example.com` |
| `{expire_date}` | Expiry date | `2026-10-12` |
| `{days}` | Days remaining | `14` |
| `{count}` | Count | `3` |

## 5. Automatic trigger rules

- **Schedule**: the first scan runs 30 seconds after the panel starts, then daily at **09:05**
- **Subscription expiry**: scans active subscriptions (`status=active`) expiring within 7 days; the customer must also be active
- **Low balance**: scans active customers with balance ≤ threshold; the threshold is `notify.balance_threshold`, **in cents** (default `100` = 1.00)
- **Deduplication**: at most one message per customer per event **per day** (`notification_send_logs` dedupes by day)

## 6. Node health monitoring

Node heartbeats are checked continuously; **notifications are sent only when the state changes — rounds with no change are silent**:

| Setting key | Description | Default |
|---|---|---|
| `monitor.node_interval_seconds` | Check interval (seconds) | `60` (minimum 30) |
| `monitor.node_offline_timeout_seconds` | Heartbeat timeout (seconds) | `120` |

- **Offline alert**: when a node goes from online to offline, sends the admin template “**Node offline alert**” with `{node}` (name) and `{time}`
- **Recovery notice**: when a node recovers, sends “**Node recovery notice**” with the same variables
- **Silent rounds**: no message when the state is unchanged; after a panel restart the first round only establishes a baseline
- **Monitor without sending**: disable the corresponding template (Enabled = OFF)

## 7. Node monthly traffic quota alerts (automatic)

- **Trigger**: when a node's monthly traffic reaches the quota threshold, sends “**Node traffic quota alert**”; when it exceeds 100% and is suspended, sends an over-limit alert (event keys `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Threshold**: `monitor.traffic_quota_threshold_percent` (percentage, default `80`; set `0` to disable threshold alerts)
- **Deduplication**: at most one per node per billing cycle (the cycle follows the node's `traffic_quota_reset_day`)
- Available variables: `{node}` (name), `{used}` (GB used), `{quota}` (GB quota), `{percent}` (percentage), `{time}`

## 8. Backup result notifications (automatic)

- **Trigger**: sent immediately after a scheduled backup (`BackupScheduler`) runs — success sends “**Daily backup report**”, failure (creating the backup / reading storage / uploading to remote) sends “**Backup failure alert**”
- Available variables: `{backup_file}` (name), `{size}` (size, success report only), `{message}` (failure reason, failure alert only), `{time}`
- **Monitor without sending**: silently skipped when the template is disabled (Enabled = OFF)

## 9. Certificate expiry reminders (automatic)

- **Trigger**: daily at 09:05, scans `active` certificates nearing expiry (**IP certificates with 3 days left**, **domain certificates with 7 days left**) and sends the admin template “**Certificate expiry reminder**” per certificate
- **Deduplication**: at most one reminder per certificate per day; after renewal the expiry moves outside the window (IP 3 days / domain 7 days) and reminders stop
- Available variables: `{domain}` (certificate domain), `{expire_date}` (expiry date), `{days}` (days remaining), `{time}`

## 10. Core process crash alerts (automatic)

- **Trigger**: when a core process (xray/singbox) exits abnormally or the proxy process (caddy/nginx) crashes, sends the admin template “**Core process crash alert**” immediately; the supervisor then restarts it with exponential backoff
- Available variables: `{core}` (process type), `{restarts}` / `{max}` (restart count / limit), `{message}` (exit message, core processes only), `{time}`
- **Monitor without sending**: silently skipped when the template is disabled (Enabled = OFF)

## 11. New user registration notices (automatic)

- **Trigger**: sent immediately after a customer registers successfully, using the admin template “**New user registration notice**”
- Available variables: `{user}` (customer email), `{time}`

## 12. Usage

1. **Add a template**: Notification settings → Notification templates → Add template → “Fill from preset” and adjust as needed
2. **Enable**: tick Enabled and save; automatic triggers only process enabled templates
3. **Test**: click “Send” in the list to send a real message with example variables (subscriber template test emails go to the administrator's sender address)
4. **Immediate notifications** (business code): call `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` at the event site (recharge, deduction, renewal) to notify a single customer immediately, outside the daily scan limit
