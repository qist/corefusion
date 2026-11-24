package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/process"
	"corefusion/internal/store/repository"
)

// pushRecorder 记录 PushCoreGuardians 的调用（守护设置下发必须被触发）。
type pushRecorder struct {
	mu   sync.Mutex
	ids  []int64
	call chan int64
}

type guardianTestOperator struct {
	rec   *pushRecorder
	local bool
}

func (o *guardianTestOperator) IsLocalServer(context.Context, int64) bool { return o.local }
func (o *guardianTestOperator) ControlCore(context.Context, int64, string, string) error {
	return nil
}
func (o *guardianTestOperator) GetRemoteState(context.Context, int64) (map[string]*CoreStateInfo, []*BinaryInfoItem, error) {
	return nil, nil, nil
}
func (o *guardianTestOperator) PushCoreGuardians(_ context.Context, serverID int64) error {
	o.rec.mu.Lock()
	o.rec.ids = append(o.rec.ids, serverID)
	o.rec.mu.Unlock()
	o.rec.call <- serverID
	return nil
}

func newGuardianHandler(t *testing.T, local bool) (*ProcessHandler, *pushRecorder, int64) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "guardian.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.InboundService{}, &entity.ProcessGuardian{}, &entity.CoreCrashLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &entity.InboundService{ServerID: 12, CoreType: "singbox", Name: "sing-box"}
	if err := db.Create(svc).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	rec := &pushRecorder{call: make(chan int64, 4)}
	mgr := process.NewManager(t.TempDir(),
		repository.NewServiceRepository(db),
		repository.NewProcessGuardianRepository(db),
		repository.NewCrashLogRepository(db),
		nil,
	)
	h := &ProcessHandler{
		processMgr:   mgr,
		svcRepo:      repository.NewServiceRepository(db),
		nodeOperator: &guardianTestOperator{rec: rec, local: local},
	}
	return h, rec, svc.ID
}

// 远端节点改守护后必须立刻下发，否则「面板显示已开启、kill 后不拉起」。
func TestUpdateGuardianPushesToRemoteNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, rec, serviceID := newGuardianHandler(t, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut,
		"/api/v1/node-servers/12/services/7/guardian",
		strings.NewReader(`{"auto_start":true,"auto_restart":true,"max_restart_count":5}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "12"}, {Key: "serviceId", Value: itoa(serviceID)}}
	h.UpdateGuardian(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	select {
	case id := <-rec.call:
		if id != 12 {
			t.Fatalf("pushed server = %d, want 12", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("远端节点改守护后没有触发 PushCoreGuardians（Agent 永远收不到新设置）")
	}

	// 确认面板库里的守护值确实写入了（下发内容来自这张表）。
	g, err := h.processMgr.GetGuardian(context.Background(), serviceID)
	if err != nil || g == nil {
		t.Fatalf("guardian not saved: %v", err)
	}
	if !g.AutoStart || !g.AutoRestart {
		t.Fatalf("guardian saved wrong: %+v", g)
	}
}

// 本地节点（面板自己）不需要下发：守护直接作用于本地进程管理器。
func TestUpdateGuardianSkipsLocalNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, rec, serviceID := newGuardianHandler(t, true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut,
		"/api/v1/node-servers/1/services/7/guardian",
		strings.NewReader(`{"auto_restart":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "serviceId", Value: itoa(serviceID)}}
	h.UpdateGuardian(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	select {
	case id := <-rec.call:
		t.Fatalf("本地节点不应下发守护，实际推给 server %d", id)
	case <-time.After(300 * time.Millisecond):
		// ok
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
