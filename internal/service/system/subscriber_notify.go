package system

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// ============================================================
// 订阅者（客户）通知 —— 与面板使用者的运维通知分离：
//   admin      模板：发到管理员的 Telegram / SMTP 收件人（现有渠道）
//   subscriber 模板：按客户邮箱经 SMTP 发给客户本人（余额/到期等业务提醒）
//
// 自动触发：每日扫描「订阅到期」「余额不足」两类事件，用
// notification_send_logs 按（事件, 客户, 天）去重，避免重复轰炸。
// ============================================================

const (
	EventSubscriptionExpire = "subscription_expire"
	EventBalanceLow         = "balance_low"

	balanceThresholdSetting = "notify.balance_threshold" // 单位：分
	defaultBalanceThreshold = int64(100)                 // 1.00 元

	notifyExpireAheadDays = 7
	notifyScanHour        = 9
	notifyScanMinute      = 5
)

// renderVars 把文本中的 {key} 占位符替换为 vars（与 SendTemplateNotification 同一套约定）。
func renderVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

// SendSubscriberTemplate 给单个客户实际发送一条订阅者模板通知（email 渠道）。
// 调用方负责 vars 的真实值；发送前不查 send_log（手动/事件直发由调用方决定）。
func (s *SettingService) SendSubscriberTemplate(ctx context.Context, tmpl *entity.NotificationTemplate, clientEmail string, vars map[string]string) error {
	if tmpl == nil {
		return fmt.Errorf("template is nil")
	}
	if tmpl.Audience != entity.TemplateAudienceSubscriber {
		return fmt.Errorf("template %q is not a subscriber template", tmpl.Name)
	}
	if tmpl.Type != "email" {
		return fmt.Errorf("订阅者模板仅支持 email 渠道（订阅者没有 Telegram 会话）")
	}
	if clientEmail == "" {
		return fmt.Errorf("客户邮箱为空")
	}
	if defaultNotifier == nil {
		return fmt.Errorf("SMTP 未配置，无法发送订阅者通知")
	}
	subject := renderVars(tmpl.Subject, vars)
	if subject == "" {
		subject = "CoreFusion Notification"
	}
	return defaultNotifier.SendEmail(ctx, clientEmail, subject, renderVars(tmpl.Content, vars))
}

// SendSubscriberTemplateTest 以管理员邮箱为收件人演示发送订阅者模板（验证内容渲染与 SMTP 连通）。
func (s *SettingService) SendSubscriberTemplateTest(ctx context.Context, tmpl *entity.NotificationTemplate) error {
	config, err := s.GetNotificationConfig(ctx)
	if err != nil {
		return err
	}
	if config.SMTPFrom == "" {
		return fmt.Errorf("请先配置 SMTP 与发件人邮箱，演示邮件将发送到发件人邮箱")
	}
	return s.SendSubscriberTemplate(ctx, tmpl, config.SMTPFrom, sampleTemplateVars)
}

// markSubscriberNotified 记录发送（用于按天去重）。写入失败仅记日志，不阻断。
func markSubscriberNotified(db *gorm.DB, tmplID int64, eventKey, clientEmail, day string) {
	if err := db.Create(&entity.NotificationSendLog{
		TemplateID: tmplID, EventKey: eventKey, ClientKey: clientEmail, Day: day,
	}).Error; err != nil {
		log.Printf("[subscriber-notify] write send log failed: %v", err)
	}
}

// alreadyNotifiedToday 查询今天是否已给该客户发过该事件的通知。
func alreadyNotifiedToday(db *gorm.DB, eventKey, clientEmail, day string) bool {
	var count int64
	db.Model(&entity.NotificationSendLog{}).
		Where("event_key = ? AND client_key = ? AND day = ?", eventKey, clientEmail, day).
		Count(&count)
	return count > 0
}

// ScanAndNotifySubscribers 执行一次订阅者通知扫描（订阅到期 + 余额不足），返回各事件发送条数。
func ScanAndNotifySubscribers(ctx context.Context, db *gorm.DB, settingSvc *SettingService) (expireSent, balanceSent int) {
	day := time.Now().Format("2006-01-02")

	// ---- 1. 订阅到期提醒（7 天内到期）----
	var expireTemplates []entity.NotificationTemplate
	db.Where("audience = ? AND type = ? AND event_key = ? AND enabled = ?",
		entity.TemplateAudienceSubscriber, "email", EventSubscriptionExpire, true).Find(&expireTemplates)
	if len(expireTemplates) > 0 {
		var rows []struct {
			Email     string     `gorm:"column:email"`
			ExpiresAt *time.Time `gorm:"column:expires_at"`
		}
		db.Table("subscriptions").
			Select("clients.email AS email, subscriptions.expires_at AS expires_at").
			Joins("JOIN clients ON clients.id = subscriptions.client_id").
			Where("subscriptions.status = ? AND clients.status = ? AND subscriptions.expires_at IS NOT NULL", "active", "active").
			Where("subscriptions.expires_at BETWEEN ? AND ?", time.Now(), time.Now().AddDate(0, 0, notifyExpireAheadDays)).
			Scan(&rows)
		for _, r := range rows {
			if r.Email == "" || r.ExpiresAt == nil {
				continue
			}
			if alreadyNotifiedToday(db, EventSubscriptionExpire, r.Email, day) {
				continue
			}
			days := int(time.Until(*r.ExpiresAt).Hours() / 24)
			if days < 0 {
				days = 0
			}
			vars := map[string]string{
				"user":        r.Email,
				"expire_date": r.ExpiresAt.Format("2006-01-02 15:04"),
				"days":        fmt.Sprintf("%d", days),
				"time":        time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := settingSvc.SendSubscriberTemplate(ctx, &expireTemplates[0], r.Email, vars); err != nil {
				log.Printf("[subscriber-notify] expire notice to %s failed: %v", r.Email, err)
				continue
			}
			markSubscriberNotified(db, expireTemplates[0].ID, EventSubscriptionExpire, r.Email, day)
			expireSent++
		}
	}

	// ---- 2. 余额不足提醒 ----
	var balanceTemplates []entity.NotificationTemplate
	db.Where("audience = ? AND type = ? AND event_key = ? AND enabled = ?",
		entity.TemplateAudienceSubscriber, "email", EventBalanceLow, true).Find(&balanceTemplates)
	if len(balanceTemplates) > 0 {
		threshold := defaultBalanceThreshold
		if v, err := settingSvc.GetByKey(ctx, balanceThresholdSetting); err == nil && v != "" {
			if n, perr := parseBalanceThreshold(v); perr == nil {
				threshold = n
			}
		}
		var rows []struct {
			Email   string `gorm:"column:email"`
			Balance int64  `gorm:"column:balance"`
		}
		db.Table("client_balances").
			Select("clients.email AS email, client_balances.balance AS balance").
			Joins("JOIN clients ON clients.id = client_balances.client_id").
			Where("clients.status = ? AND client_balances.balance <= ?", "active", threshold).
			Scan(&rows)
		for _, r := range rows {
			if r.Email == "" {
				continue
			}
			if alreadyNotifiedToday(db, EventBalanceLow, r.Email, day) {
				continue
			}
			vars := map[string]string{
				"user":    r.Email,
				"balance": fmt.Sprintf("%.2f", float64(r.Balance)/100),
				"time":    time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := settingSvc.SendSubscriberTemplate(ctx, &balanceTemplates[0], r.Email, vars); err != nil {
				log.Printf("[subscriber-notify] balance notice to %s failed: %v", r.Email, err)
				continue
			}
			markSubscriberNotified(db, balanceTemplates[0].ID, EventBalanceLow, r.Email, day)
			balanceSent++
		}
	}
	return expireSent, balanceSent
}

func parseBalanceThreshold(v string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(v, "%d", &n)
	return n, err
}

// StartSubscriberNotifyLoop 启动每日扫描循环：启动 30 秒后先跑一次（首次立即感知），
// 之后每天 09:05 执行（随机事件如续费/充值建议由业务代码在变更点直接调用
// NotifyClientEvent 即时提醒，不依赖本循环）。
func StartSubscriberNotifyLoop(ctx context.Context, db *gorm.DB, settingSvc *SettingService) {
	run := func() {
		e, b := ScanAndNotifySubscribers(ctx, db, settingSvc)
		if e > 0 || b > 0 {
			log.Printf("[subscriber-notify] scanned: %d expire notices, %d balance notices", e, b)
		}
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		run()
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), notifyScanHour, notifyScanMinute, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(next.Sub(now)):
				run()
			}
		}
	}()
}

// NotifyClientEvent 供业务代码在事件发生点（充值、扣费、续费等）即时给单个客户发送
// 订阅者模板：按 event_key 查找启用的 subscriber 模板并发送，模板不存在/未启用则静默跳过。
func NotifyClientEvent(ctx context.Context, db *gorm.DB, settingSvc *SettingService, eventKey, clientEmail string, vars map[string]string) {
	var tmpl entity.NotificationTemplate
	if err := db.Where("audience = ? AND event_key = ? AND enabled = ?",
		entity.TemplateAudienceSubscriber, eventKey, true).First(&tmpl).Error; err != nil {
		return
	}
	if err := settingSvc.SendSubscriberTemplate(ctx, &tmpl, clientEmail, vars); err != nil {
		log.Printf("[subscriber-notify] event %s to %s failed: %v", eventKey, clientEmail, err)
	}
}

// ---- 手动群发（营销/召回类）----

// BroadcastSubscriberTemplate 向一批客户群发订阅者模板（管理员手动触发，如广告/召回邮件）。
// 显式 emails 优先（手动勾选客户）；否则按 scope 决定收件人范围：
//   - "active":    仅正常账号（自动提醒类默认范围）
//   - "all":       全部账号（含禁用 —— 广告类营销可发）
//   - "suspended": 仅禁用账号（召回）
//
// 发送在后台 goroutine 中逐个执行（{user} 替换为客户邮箱），返回预计发送人数。
func (s *SettingService) BroadcastSubscriberTemplate(ctx context.Context, db *gorm.DB, id int64, scope string, emails []string) (int, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	if tmpl == nil {
		return 0, fmt.Errorf("template %d not found", id)
	}
	if tmpl.Audience != entity.TemplateAudienceSubscriber || tmpl.Type != "email" {
		return 0, fmt.Errorf("仅订阅者邮件模板支持群发")
	}

	var targets []entity.Client
	if len(emails) > 0 {
		// 手动勾选的客户：按邮箱查询（校验存在）
		if err := db.Select("email").Where("email IN ?", emails).Find(&targets).Error; err != nil {
			return 0, err
		}
	} else {
		query := db.Model(&entity.Client{})
		switch scope {
		case "all":
			// 全部账号（含禁用/过期 —— 广告类营销可发）
		case "suspended":
			query = query.Where("status <> ?", "active")
		default:
			scope = "active"
			query = query.Where("status = ?", "active")
		}
		if err := query.Select("email").Find(&targets).Error; err != nil {
			return 0, err
		}
	}

	count := len(targets)
	if count == 0 {
		return 0, nil
	}
	if defaultNotifier == nil {
		return 0, fmt.Errorf("SMTP 未配置，无法群发")
	}

	go func() {
		bgCtx := context.Background()
		sent, failed := 0, 0
		for _, c := range targets {
			if c.Email == "" {
				continue
			}
			vars := map[string]string{"user": c.Email, "time": time.Now().Format("2006-01-02 15:04:05")}
			if err := s.SendSubscriberTemplate(bgCtx, tmpl, c.Email, vars); err != nil {
				failed++
				log.Printf("[subscriber-notify] broadcast to %s failed: %v", c.Email, err)
				continue
			}
			sent++
			time.Sleep(100 * time.Millisecond) // 轻微限速，避免 SMTP 触发频率限制
		}
		log.Printf("[subscriber-notify] broadcast done: template=%q scope=%s sent=%d failed=%d", tmpl.Name, scope, sent, failed)
	}()
	return count, nil
}
