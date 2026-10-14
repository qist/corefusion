package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// ============================================================
// 证书到期监控 —— 每日扫描「N 天内到期的 active 证书」，逐张发 admin 模板
// 「证书到期提醒」（未启用时 NotifyTemplate 静默 no-op）。
//   扫描窗口：按证书类型区分 ——
//     · IP 证书（ACME shortlived，约 6 天寿命）：剩 3 天起提醒；
//     · 域名证书（默认）：剩 7 天起提醒（与订阅到期提醒一致）。
//   去重：notification_send_logs 按（事件 cert_expire, 证书域名, 天）去重，
//         即到期前每天最多提醒一次，续期后（到期日推迟出窗口）自动停止。
// ============================================================

const (
	certExpiryEventKey = "cert_expire"
	// 域名证书（默认）提前 7 天提醒。
	certExpiryAheadDays = 7
	// IP 证书是 ACME 短时证书（约 6 天寿命）→ 只剩 3 天时才开始提醒，
	// 与面板自动续期阈值（PublicCertIPRenewBefore=3 天）同口径，避免 7 天窗口下
	// 长期刷屏「还有 6 天到期」这类噪声。
	certExpiryIPAheadDays = 3
	tmplNameCertExpiry    = "证书到期提醒"
)

// certExpiryScanWindowDays 是数据库扫描窗口：取两类阈值中较大的那个，
// 否则只按 IP 的 3 天窗口查会漏掉域名证书（7 天）。
func certExpiryScanWindowDays() int {
	if certExpiryAheadDays > certExpiryIPAheadDays {
		return certExpiryAheadDays
	}
	return certExpiryIPAheadDays
}

// certExpiryDaysFor 返回该证书的到期提醒窗口（天）：
// 含 IP 标识符 → 按 IP 短时证书 3 天；否则按域名证书 7 天。
func certExpiryDaysFor(cert entity.Certificate) int {
	for _, id := range certIdentifierList(cert.Domains, cert.Domain) {
		if net.ParseIP(strings.TrimSpace(id)) != nil {
			return certExpiryIPAheadDays
		}
	}
	return certExpiryAheadDays
}

// certIdentifierList 取证书的标识符：优先 domains（JSON 数组），退化为 domain。
func certIdentifierList(domains, domain string) []string {
	out := make([]string, 0, 2)
	if strings.TrimSpace(domains) != "" {
		var arr []string
		if err := json.Unmarshal([]byte(domains), &arr); err == nil {
			for _, d := range arr {
				if d = strings.TrimSpace(d); d != "" {
					out = append(out, d)
				}
			}
		}
	}
	if len(out) == 0 && strings.TrimSpace(domain) != "" {
		out = append(out, strings.TrimSpace(domain))
	}
	return out
}

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
		Where("status = ? AND expires_at BETWEEN ? AND ?", "active", time.Now(), time.Now().AddDate(0, 0, certExpiryScanWindowDays())).
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
		days := int(time.Until(cert.ExpiresAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		// 按证书类型决定提醒窗口：IP 短时证书 3 天、域名证书 7 天。
		// 数据库扫描窗口取的是两者较大的 7 天，所以还需在这里把"IP 证书但只剩 >3 天"
		// 的提前剔除 —— 否则 7 天窗口下 IP 证书会从"还剩 6 天"就开始天天提醒。
		if days > certExpiryDaysFor(cert) {
			continue
		}
		if alreadyNotifiedToday(db, certExpiryEventKey, cert.Domain, day) {
			continue
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
