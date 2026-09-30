package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// IP 黑名单（P3-4，运维配置的面板级策略）：
//   - 配置存储：system_settings 表 key=panel.ip_blacklist，每行一条（IP 或 CIDR，# 开头为注释）
//   - 管理入口：管理端 Settings.vue → 安全 → IP 黑名单（运维维护，非客户自助）
//   - 生效面：**客户端面** —— 订阅拉取（8081 /:token、8080 /sub/:token）+ 门户登录/注册/OAuth；
//     管理 API **不拦**（运维误把自己 IP 拉黑时不至于锁死管理入口）
//
// 实现：30s TTL 缓存读取设置（避免每请求查库），空列表零开销直通。
type ipBlacklistCache struct {
	mu        sync.RWMutex
	rules     []*net.IPNet
	ips       []net.IP
	loadedAt  time.Time
	loadError bool
}

var globalIPBlacklist ipBlacklistCache

const ipBlacklistSettingKey = "panel.ip_blacklist"

// parseIPBlacklist 解析黑名单配置文本：每行/逗号分隔一条，支持 IP 与 CIDR；非法行忽略。
func parseIPBlacklist(raw string) (rules []*net.IPNet, ips []net.IP) {
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' || r == ' ' || r == '\r' || r == ';' }) {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		// CIDR（如 203.0.113.0/24）
		if strings.Contains(entry, "/") {
			if _, ipnet, err := net.ParseCIDR(entry); err == nil {
				rules = append(rules, ipnet)
			}
			continue
		}
		// 单 IP
		if ip := net.ParseIP(entry); ip != nil {
			ips = append(ips, ip)
		}
	}
	return rules, ips
}

// loadIPBlacklist 读取设置并刷新缓存（TTL 30s；并发下允许偶发重复加载，无锁竞争风险）。
func loadIPBlacklist(db *gorm.DB) {
	globalIPBlacklist.mu.RLock()
	fresh := time.Since(globalIPBlacklist.loadedAt) < 30*time.Second
	globalIPBlacklist.mu.RUnlock()
	if fresh {
		return
	}

	globalIPBlacklist.mu.Lock()
	defer globalIPBlacklist.mu.Unlock()
	// 双检：拿写锁期间可能已被别的请求刷新
	if time.Since(globalIPBlacklist.loadedAt) < 30*time.Second {
		return
	}
	var value string
	if db != nil {
		// 复用系统设置仓储（查询口径与全局一致：backtick 引用 key 列）
		repo := repository.NewSystemSettingRepository(db)
		if v, err := repo.GetByKey(context.Background(), ipBlacklistSettingKey); err == nil {
			value = v
		}
	}
	globalIPBlacklist.rules, globalIPBlacklist.ips = parseIPBlacklist(value)
	globalIPBlacklist.loadedAt = time.Now()
}

// isIPBlacklisted 判定 IP 是否命中黑名单（IP 精确 + CIDR 网段）。
func isIPBlacklisted(ipStr string) bool {
	if ipStr == "" {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	globalIPBlacklist.mu.RLock()
	defer globalIPBlacklist.mu.RUnlock()
	if len(globalIPBlacklist.rules) == 0 && len(globalIPBlacklist.ips) == 0 {
		return false
	}
	for _, bip := range globalIPBlacklist.ips {
		if bip.Equal(ip) {
			return true
		}
	}
	for _, rule := range globalIPBlacklist.rules {
		if rule.Contains(ip) {
			return true
		}
	}
	return false
}

// IPBlacklistGuard 客户端面 IP 黑名单拦截（见文件头注释）。
// 用真实客户端 IP（经可信代理解析，防伪造）匹配黑名单，命中返回 403。
func IPBlacklistGuard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loadIPBlacklist(db)
		if isIPBlacklisted(netutil.GetRealIP(c.Request)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "IP_BLACKLISTED",
				"message": "your IP is blocked by the administrator",
			})
			return
		}
		c.Next()
	}
}
