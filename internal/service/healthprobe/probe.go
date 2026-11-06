// Package healthprobe 探测「CF 优选域名 / CF 优选 IP」的可达性。
//
// 设计口径（与用户确认）：
//   - Enabled 是**管理员开关**，Healthy 是**探测结论** —— 两者分开，
//     探测失败只标记不健康，不改用户开关，避免误操作不可逆；
//   - 连续失败 3 次才判定不健康（一次抖动不下线），成功即恢复；
//   - 订阅渲染只挑健康条目，但如果某类**全部不健康**则回退全量，
//     防止一次网络故障把订阅里的地址全清空。
//
// 优选 IP 只探 TCP 443（Cloudflare 对裸 IP 做 TLS 常被重置，TCP 可达即可用）；
// 优选域名走 TLS 握手（域名过 CF 代理，TCP 通但证书/SNI 错也说明不可用）。
package healthprobe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

const (
	probeTimeout  = 3 * time.Second
	failThreshold = 3
	concurrency   = 8
)

// Result 是一次全量探测的汇总（供接口返回/日志）。
type Result struct {
	Checked   int `json:"checked"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Skipped   int `json:"skipped"`
}

// Prober 负责优选域名/IP 的探活。
type Prober struct {
	cfRepo    *repository.CFIPEntryRepository
	shareRepo *repository.ShareDomainConfigRepository

	// mu 防止手动检测与定时检测同时跑（会互相覆盖 fail_count）。
	mu sync.Mutex
}

// New 创建探测器。
func New(cfRepo *repository.CFIPEntryRepository, shareRepo *repository.ShareDomainConfigRepository) *Prober {
	return &Prober{cfRepo: cfRepo, shareRepo: shareRepo}
}

// StartScheduler 周期性探测（默认 5 分钟）。
func (p *Prober) StartScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		// 启动先跑一次，老数据也能尽快有状态
		if res, err := p.CheckAll(ctx); err != nil {
			log.Printf("[healthprobe] initial check failed: %v", err)
		} else {
			log.Printf("[healthprobe] initial check: checked=%d healthy=%d unhealthy=%d skipped=%d",
				res.Checked, res.Healthy, res.Unhealthy, res.Skipped)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if res, err := p.CheckAll(ctx); err != nil {
					log.Printf("[healthprobe] scheduled check failed: %v", err)
				} else if res.Unhealthy > 0 {
					log.Printf("[healthprobe] checked=%d healthy=%d unhealthy=%d", res.Checked, res.Healthy, res.Unhealthy)
				}
			}
		}
	}()
}

// CheckAll 探测全部优选 IP 与优选域名。
func (p *Prober) CheckAll(ctx context.Context) (Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var res Result
	var wg sync.WaitGroup
	var resMu sync.Mutex
	sem := make(chan struct{}, concurrency)

	record := func(healthy bool) {
		resMu.Lock()
		res.Checked++
		if healthy {
			res.Healthy++
		} else {
			res.Unhealthy++
		}
		resMu.Unlock()
	}
	skip := func() {
		resMu.Lock()
		res.Skipped++
		resMu.Unlock()
	}

	var firstErr error
	setErr := func(err error) {
		resMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		resMu.Unlock()
	}

	// 1) 优选 IP（cf_ip_entries）
	if p.cfRepo != nil {
		entries, err := p.cfRepo.List(ctx, 0)
		if err != nil {
			setErr(err)
		} else {
			for i := range entries {
				e := entries[i]
				if !e.Enabled || strings.TrimSpace(e.IP) == "" {
					skip()
					continue
				}
				port := firstPort(e.Ports, 443)
				wg.Add(1)
				sem <- struct{}{}
				go func(e entity.CFIPEntry, port int) {
					defer wg.Done()
					defer func() { <-sem }()
					ok, latency := probeTCP(e.IP, port)
					healthy, fail := nextState(ok, e.FailCount)
					if err := p.cfRepo.UpdateHealth(ctx, e.ID, healthy, latency, fail, time.Now()); err != nil {
						log.Printf("[healthprobe] update cf ip %s failed: %v", e.IP, err)
					}
					record(healthy)
				}(e, port)
			}
		}
	}

	// 2) 优选域名（share_domain_configs 里 type=cf_domain）
	if p.shareRepo != nil {
		domains, err := p.shareRepo.List(ctx, "cf_domain")
		if err != nil {
			setErr(err)
		} else {
			for i := range domains {
				d := domains[i]
				if !d.Enabled || strings.TrimSpace(d.Value) == "" {
					skip()
					continue
				}
				port := firstPort(d.Ports, d.Port)
				wg.Add(1)
				sem <- struct{}{}
				go func(d entity.ShareDomainConfig, port int) {
					defer wg.Done()
					defer func() { <-sem }()
					ok, latency := probeTLS(d.Value, port)
					healthy, fail := nextState(ok, d.FailCount)
					if err := p.shareRepo.UpdateHealth(ctx, d.ID, healthy, latency, fail, time.Now()); err != nil {
						log.Printf("[healthprobe] update share domain %s failed: %v", d.Value, err)
					}
					record(healthy)
				}(d, port)
			}
		}
	}

	wg.Wait()
	return res, firstErr
}

// nextState 根据本次结果推导 healthy / 连续失败次数。
// 连续失败达到阈值才判不健康；任一次成功立即恢复。
func nextState(ok bool, prevFail int) (healthy bool, failCount int) {
	if ok {
		return true, 0
	}
	failCount = prevFail + 1
	return failCount < failThreshold, failCount
}

// firstPort 从 JSON 端口数组里取第一个端口；取不到时回落 fallback。
func firstPort(portsJSON string, fallback int) int {
	if strings.TrimSpace(portsJSON) != "" {
		var ports []int
		if json.Unmarshal([]byte(portsJSON), &ports) == nil && len(ports) > 0 && ports[0] > 0 {
			return ports[0]
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 443
}

// probeTCP 测 TCP 可达性，返回毫秒延迟。
func probeTCP(host string, port int) (bool, int) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, int(time.Since(start).Milliseconds())
}

// probeTLS 对优选域名做 TLS 握手（跳过证书校验：CF 边缘证书常与本机信任链无关）。
func probeTLS(host string, port int) (bool, int) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: probeTimeout}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, int(time.Since(start).Milliseconds())
}
