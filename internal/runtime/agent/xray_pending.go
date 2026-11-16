package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NotConnectedError 表示目标节点的 Agent 连接不存在（断连 / 尚未连上）。
// 用类型而不是纯字符串，便于上层用 errors.As 判断「这次失败是控制面不可达」。
// Error 文案与历史保持一致（UI/日志里已有 "server N not connected"）。
type NotConnectedError struct {
	ServerID int64
}

func (e *NotConnectedError) Error() string {
	return fmt.Sprintf("server %d not connected", e.ServerID)
}

// XrayPendingKind 标识一条待补下发的 xray 运行态目标。
type XrayPendingKind string

const (
	// XrayPendingInbound 把某个入站同步到数据库的期望状态：
	// 库里存在且启用 → 覆盖下发；否则从运行态移除。
	// 入站覆盖会连带该入站下的所有用户，因此「加/删用户」也归到这一类。
	XrayPendingInbound XrayPendingKind = "inbound"
	// XrayPendingOutbound 把某个出站同步到数据库的期望状态（同上语义）。
	XrayPendingOutbound XrayPendingKind = "outbound"
	// XrayPendingRouting 全量重放路由规则（路由没有按 tag 增删的语义）。
	XrayPendingRouting XrayPendingKind = "routing"
)

// XrayPendingOp 是本进程内存里的一条待补下发记录（不落库）。
//
// 它只在「控制面不可达导致热更失败」时产生——那种失败下改动已经落盘，
// 但运行中的 xray 还是旧的；Agent 重连后按这里的记录把运行态补回来，
// 避免出现「磁盘是新配置、进程跑的是旧配置」且周期同步（只写文件不重启）救不回来。
type XrayPendingOp struct {
	Kind XrayPendingKind
	Tag  string
}

func pendingKey(op XrayPendingOp) string {
	if op.Kind == XrayPendingRouting {
		return string(op.Kind)
	}
	return string(op.Kind) + ":" + op.Tag
}

// xrayPendingQueue 按节点维护待补下发操作，同一目标只保留最后一条。
//
// 同一 (kind, tag) 反复记录没有意义：补下发是按「数据库期望状态」同步该 tag，
// 不关心中间经历过多少次增删。
type xrayPendingQueue struct {
	mu  sync.Mutex
	ops map[int64][]XrayPendingOp
}

func (q *xrayPendingQueue) add(serverID int64, op XrayPendingOp) {
	if serverID <= 0 || op.Kind == "" {
		return
	}
	if op.Kind != XrayPendingRouting && strings.TrimSpace(op.Tag) == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ops == nil {
		q.ops = make(map[int64][]XrayPendingOp)
	}
	key := pendingKey(op)
	list := q.ops[serverID]
	for i := range list {
		if pendingKey(list[i]) == key {
			list[i] = op
			return
		}
	}
	q.ops[serverID] = append(list, op)
}

func (q *xrayPendingQueue) take(serverID int64) []XrayPendingOp {
	q.mu.Lock()
	defer q.mu.Unlock()
	list := q.ops[serverID]
	delete(q.ops, serverID)
	return list
}

// restore 把补下发失败的操作放回队列，与期间新增的记录按目标去重。
func (q *xrayPendingQueue) restore(serverID int64, ops []XrayPendingOp) {
	for _, op := range ops {
		q.add(serverID, op)
	}
}

func (q *xrayPendingQueue) len(serverID int64) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.ops[serverID])
}

// XrayPendingReplayer 在 Agent 重连后把待补下发的运行态写入目标节点。
// 实现方在 router 包（读数据库期望状态 → 经 ControlProvider 下发）。
type XrayPendingReplayer interface {
	ReplayXrayPending(ctx context.Context, serverID int64, ops []XrayPendingOp) error
}

// SetXrayPendingReplayer 注入补下发实现。
func (m *NodeServerManager) SetXrayPendingReplayer(r XrayPendingReplayer) {
	m.xrayPendingReplayer = r
}

// MarkXrayPending 记录一条待补下发（仅远端节点；本地节点没有 Agent 重连事件）。
func (m *NodeServerManager) MarkXrayPending(serverID int64, op XrayPendingOp) {
	if m == nil || serverID <= 0 || serverID == m.localServerID {
		return
	}
	m.xrayPending.add(serverID, op)
}

// XrayPendingCount 返回某节点当前待补下发的条数（测试/诊断用）。
func (m *NodeServerManager) XrayPendingCount(serverID int64) int {
	if m == nil {
		return 0
	}
	return m.xrayPending.len(serverID)
}

// TakeXrayPending 取走某节点的待补下发列表（取走即清空，失败时用 Restore 放回）。
func (m *NodeServerManager) TakeXrayPending(serverID int64) []XrayPendingOp {
	if m == nil {
		return nil
	}
	return m.xrayPending.take(serverID)
}

// RestoreXrayPending 把补下发失败的操作放回队列（与期间新增的记录自动去重）。
func (m *NodeServerManager) RestoreXrayPending(serverID int64, ops []XrayPendingOp) {
	if m == nil {
		return
	}
	m.xrayPending.restore(serverID, ops)
}

// replayPendingXray 在 Agent 重连后取走待补队列并下发；失败放回队列等下次重连再试。
func (m *NodeServerManager) replayPendingXray(serverID int64) {
	if m == nil || m.xrayPendingReplayer == nil || serverID == m.localServerID {
		return
	}
	ops := m.TakeXrayPending(serverID)
	if len(ops) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := m.xrayPendingReplayer.ReplayXrayPending(ctx, serverID, ops); err != nil {
		m.RestoreXrayPending(serverID, ops)
		log.Printf("[NodeServerManager] xray pending reapply to server %d failed (%d ops, kept for next reconnect): %v",
			serverID, len(ops), err)
		return
	}
	log.Printf("[NodeServerManager] xray pending reapply to server %d: %d ops applied", serverID, len(ops))
}

// IsXrayControlUnavailable 判断错误是否属于「控制面不可达」——只有这类失败
// 才值得记成待补下发。xray 自身的业务错误（handler not found、参数错误等）
// 说明指令已经送达，重连后重放也不会成功，不该记。
func IsXrayControlUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var notConnected *NotConnectedError
	if errors.As(err, &notConnected) {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"not connected",
		"connection refused",
		"connection reset",
		"transport is closing",
		"connection error",
		"broken pipe",
		"no route to host",
		"i/o timeout",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
