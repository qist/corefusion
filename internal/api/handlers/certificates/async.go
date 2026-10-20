package certificates

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/audit"
)

// 证书签发/续期是**长任务** ✓：dns-01 每个授权都要等 DNS 传播（实测一次 60s 上下，
// 慢的时候到 2 分钟 ✓），多域名 + 双证书串行更容易超过面板/反代/浏览器的 5 分钟超时 ✗。
//
// 因此面板本机的签发/续期改为**后台执行 + 任务查询** ✓：
//   - POST /certificates/issue、/certificates/:id/renew 立即返回 202 + task_id ✓；
//   - GET  /certificate-tasks/:id 查询 running/success/failed ✓；
//   - 前端提交后轮询，完成后刷新列表并提示失败原因 ✓。
//
// 任务只保存在内存里（进程重启丢失 ✓）——它只是"本次请求的结果通知" ✓，
// 权威状态始终在 certificates 表 ✓，重启后按列表/过期时间重新判断即可 ✓。

const (
	certTaskRunning = "running"
	certTaskSuccess = "success"
	certTaskFailed  = "failed"

	certTaskTTL = time.Hour
)

// CertTask 是一次后台签发/续期任务的公开状态。
type CertTask struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"` // issue | renew
	State      string     `json:"state"`
	CertID     int64      `json:"cert_id,omitempty"`
	Domain     string     `json:"domain,omitempty"`
	KeyType    string     `json:"key_type,omitempty"`
	KeyBits    int        `json:"key_bits,omitempty"`
	Message    string     `json:"message,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (t CertTask) clone() CertTask {
	c := t
	if t.FinishedAt != nil {
		ft := *t.FinishedAt
		c.FinishedAt = &ft
	}
	return c
}

type certTaskStore struct {
	mu    sync.Mutex
	tasks map[string]*CertTask
}

func newCertTaskStore() *certTaskStore {
	return &certTaskStore{tasks: make(map[string]*CertTask, 8)}
}

func newCertTaskID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

func (s *certTaskStore) create(kind, domain, keyType string, keyBits int) CertTask {
	t := &CertTask{
		ID:        newCertTaskID(),
		Kind:      kind,
		State:     certTaskRunning,
		Domain:    domain,
		KeyType:   keyType,
		KeyBits:   keyBits,
		CreatedAt: time.Now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(t.CreatedAt)
	s.tasks[t.ID] = t
	return t.clone()
}

func (s *certTaskStore) finish(id string, certID int64, err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return
	}
	t.FinishedAt = &now
	t.CertID = certID
	if err != nil {
		t.State = certTaskFailed
		t.Error = err.Error()
	} else {
		t.State = certTaskSuccess
		t.Message = "证书已签发并入库"
	}
}

func (s *certTaskStore) get(id string) (CertTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now())
	t, ok := s.tasks[id]
	if !ok {
		return CertTask{}, false
	}
	return t.clone(), true
}

// pruneLocked 清理超过 TTL 的旧任务（只在 create/get 时顺带做 ✓）。
func (s *certTaskStore) pruneLocked(now time.Time) {
	for id, t := range s.tasks {
		if now.Sub(t.CreatedAt) > certTaskTTL {
			delete(s.tasks, id)
		}
	}
}

// TaskStatus handles GET /api/v1/certificates/tasks/:id.
// 前端提交后台签发/续期后轮询这里取最终结果（含失败原因 ✓）。
func (h *Handler) TaskStatus(c *gin.Context) {
	t, ok := h.tasks.get(c.Param("id"))
	if !ok {
		response.NotFound(c, "TASK_NOT_FOUND", "task not found or expired")
		return
	}
	response.OK(c, t)
}

// taskAuditEventFromCtx 在**请求线程里**提取审计所需信息 ✓ ——
// gin.Context 不能带进后台 goroutine 使用（响应结束后会被复用 ✗）。
func taskAuditEventFromCtx(c *gin.Context, action, detail string) audit.NodeEvent {
	ev := audit.NodeEvent{
		Action:    action,
		Detail:    detail,
		SourceIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	if uid, ok := c.Get("user_id"); ok {
		if v, ok := uid.(int64); ok {
			ev.AdminID = v
		}
	}
	return ev
}
