package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ClientRepository handles client database operations.
type ClientRepository struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) List(ctx context.Context, planID int64, status string, limit, offset int) ([]entity.Client, int64, error) {
	var clients []entity.Client
	var total int64

	q := r.db.WithContext(ctx).Model(&entity.Client{})
	if planID > 0 {
		q = q.Where("plan_id = ?", planID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)

	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&clients).Error
	return clients, total, err
}

// ListByPlanID returns all active clients bound to the given plan template ID.
// Used for batch account synchronization when a new node is added to a plan.
func (r *ClientRepository) ListByPlanID(ctx context.Context, planID int64) ([]entity.Client, error) {
	var clients []entity.Client
	err := r.db.WithContext(ctx).
		Where("plan_id = ? AND status = ?", planID, "active").
		Order("id ASC").
		Find(&clients).Error
	return clients, err
}

func (r *ClientRepository) GetByID(ctx context.Context, id int64) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).First(&client, id).Error
	return &client, err
}

// GetTokenVersion 读取客户端当前令牌版本（供鉴权中间件与刷新校验）。
func (r *ClientRepository) GetTokenVersion(ctx context.Context, clientID int64) (int, error) {
	var v int
	err := r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", clientID).
		Select("COALESCE(token_version, 0)").
		Scan(&v).Error
	return v, err
}

// BumpTokenVersion 把客户端令牌版本 +1 → 此前签发的所有 token 立即失效（登出/踢下线）。
func (r *ClientRepository) BumpTokenVersion(ctx context.Context, clientID int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", clientID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

func (r *ClientRepository) GetByEmail(ctx context.Context, email string) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&client).Error
	return &client, err
}

// GetBySubscriptionID 按订阅反查客户（限速/设备数归户用）。
func (r *ClientRepository) GetBySubscriptionID(ctx context.Context, subID int64) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).Where("subscription_id = ?", subID).First(&client).Error
	return &client, err
}

// FindByIDs returns clients keyed by ID for the given IDs.
func (r *ClientRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Client, error) {
	result := make(map[int64]*entity.Client)
	if len(ids) == 0 {
		return result, nil
	}
	var clients []entity.Client
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return nil, err
	}
	for i := range clients {
		result[clients[i].ID] = &clients[i]
	}
	return result, nil
}

// SearchByEmail returns clients whose email matches the keyword (LIKE %keyword%).
func (r *ClientRepository) SearchByEmail(ctx context.Context, keyword string, limit int) ([]entity.Client, error) {
	var clients []entity.Client
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := r.db.WithContext(ctx).
		Where("email LIKE ?", "%"+keyword+"%").
		Order("id DESC").
		Limit(limit).
		Find(&clients).Error
	return clients, err
}

func (r *ClientRepository) Create(ctx context.Context, client *entity.Client) error {
	return r.db.WithContext(ctx).Create(client).Error
}

func (r *ClientRepository) Update(ctx context.Context, client *entity.Client) error {
	return r.db.WithContext(ctx).Save(client).Error
}

func (r *ClientRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Client{}, id).Error
}

// PurgeClientScopedData 删除该客户名下的所有从属数据（订阅、订单、流水、工单、推荐记录等）。
//
// 为什么要物理删除而不是只吊销：SQLite 的 AUTOINCREMENT 在表重建/清空后会复用 ID，
// 新客户拿到旧 ID 时，遗留的订单/订阅会串到新账号上（真机出现过 25 号客户看到 83 条
// 别人的订单、余额显示旧值）。删除客户必须是「带走全部从属数据」的语义。
//
// 全部在一个事务里执行：任一步失败整体回滚，不会留下半清理状态。
func (r *ClientRepository) PurgeClientScopedData(ctx context.Context, clientID int64) error {
	if clientID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stmts := []string{
			// 工单（先子表）
			`DELETE FROM ticket_messages WHERE ticket_id IN (SELECT id FROM tickets WHERE client_id = ?)`,
			`DELETE FROM tickets WHERE client_id = ?`,
			// 订阅的从属记录（先按订阅 ID 删）
			`DELETE FROM subscription_access_logs WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			`DELETE FROM subscription_rotations WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			`DELETE FROM traffic_sessions WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			`DELETE FROM traffic_usage_ledger WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			`DELETE FROM rate_limit_ip_entries WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			`DELETE FROM service_access_policies WHERE subscription_id IN (SELECT id FROM subscriptions WHERE client_id = ?)`,
			// 订阅本体
			`DELETE FROM subscriptions WHERE client_id = ?`,
			// 订单与账务
			`DELETE FROM charge_orders WHERE client_id = ?`,
			`DELETE FROM plan_orders WHERE client_id = ?`,
			`DELETE FROM balance_transactions WHERE client_id = ?`,
			`DELETE FROM client_balances WHERE client_id = ?`,
			// 其他客户级数据
			`DELETE FROM client_gifts WHERE client_id = ?`,
			`DELETE FROM client_log_packages WHERE client_id = ?`,
			`DELETE FROM client_oauth_bindings WHERE client_id = ?`,
			`DELETE FROM client_self_unban_logs WHERE client_id = ?`,
			`DELETE FROM ip_whitelist_entries WHERE client_id = ?`,
			`DELETE FROM shared_account_repair_logs WHERE client_id = ?`,
			`DELETE FROM warp_configs WHERE client_id = ?`,
			`DELETE FROM referral_records WHERE referrer_id = ? OR referee_id = ?`,
			`DELETE FROM referral_withdrawals WHERE client_id = ?`,
			`DELETE FROM referral_codes WHERE client_id = ?`,
		}
		for _, q := range stmts {
			args := []interface{}{clientID}
			if strings.Count(q, "?") == 2 {
				args = append(args, clientID)
			}
			if err := tx.Exec(q, args...).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ClientRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ClientRepository) UpdateUsage(ctx context.Context, id int64, usedBytes int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("used_bytes", usedBytes).Error
}

// AddUsedBytes 按订阅 ID 原子累加客户的已用流量。
// 与 subscriptions.used_bytes_cache 同口径：管理端「客户管理」读 clients.used_bytes，
// 而流量归户原来只写订阅缓存 → 客户列表的已用流量永远显示 0。
func (r *ClientRepository) AddUsedBytes(ctx context.Context, subscriptionID, delta int64) error {
	if subscriptionID <= 0 || delta == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("subscription_id = ?", subscriptionID).
		Update("used_bytes", gorm.Expr("used_bytes + ?", delta)).Error
}

func (r *ClientRepository) UpdateSubscriptionID(ctx context.Context, id int64, subID int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("subscription_id", subID).Error
}
