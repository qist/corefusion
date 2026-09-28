package system

import (
	"context"
	"fmt"
	"log"
	"time"

	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// ============================================================
// 证书到期监控 —— 每日扫描「N 天内到期的 active 证书」，逐张发 admin 模板
// 「证书到期提醒」（未启用时 NotifyTemplate 静默 no-op）。
//   扫描窗口：7 天（与订阅到期提醒一致）
//   去重：notification_send_logs 按（事件 cert_expire, 证书域名, 天）去重，
//         即到期前每天最多提醒一次，续期后（到期日推迟出窗口）自动停止。
// ============================================================

const (
	certExpiryEventKey  = "cert_expire"
	certExpiryAheadDays = 7
	tmplNameCertExpiry  = "证书到期提醒"
)

// StartCertExpiryMonitor 启动证书到期每日扫描（阻塞调用，请用 go 启动）：
// 启动 30 秒后先跑一次（首次立即感知），之后每天 09:05 执行。
func StartCertExpiryMonitor(ctx context.Context, db *gorm.DB) {
	run := func() {
		sent := ScanCertExpiry(ctx, db)
		if sent > 0 {
			log.Printf("[cert-expiry] scanned: %d expiring notice(s) sent", sent)
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

// ScanCertExpiry 执行一轮证书到期扫描，返回发送条数。
func ScanCertExpiry(ctx context.Context, db *gorm.DB) int {
	var certs []entity.Certificate
	if err := db.WithContext(ctx).
		Where("status = ? AND expires_at BETWEEN ? AND ?", "active", time.Now(), time.Now().AddDate(0, 0, certExpiryAheadDays)).
		Order("expires_at ASC").
		Find(&certs).Error; err != nil {
		log.Printf("[cert-expiry] scan failed: %v", err)
		return 0
	}

	day := time.Now().Format("2006-01-02")
	sent := 0
	for _, cert := range certs {
		if cert.Domain == "" {
			continue
		}
		if alreadyNotifiedToday(db, certExpiryEventKey, cert.Domain, day) {
			continue
		}
		days := int(time.Until(cert.ExpiresAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		vars := map[string]string{
			"domain":      cert.Domain,
			"expire_date": cert.ExpiresAt.Format("2006-01-02"),
			"days":        fmt.Sprintf("%d", days),
			"time":        time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := NotifyTemplate(ctx, tmplNameCertExpiry, vars); err != nil {
			log.Printf("[cert-expiry] notice for %s failed: %v", cert.Domain, err)
			continue
		}
		markSubscriberNotified(db, 0, certExpiryEventKey, cert.Domain, day)
		sent++
		log.Printf("[cert-expiry] cert %s expires in %d day(s) — notice sent", cert.Domain, days)
	}
	return sent
}
