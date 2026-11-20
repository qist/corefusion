package repository

import (
	"context"

	"gorm.io/gorm"
)

// PurgeSiteScopedData 删除支付站点的从属数据（密钥版本、回调事件、重放 nonce 等）。
//
// 为什么必须清：charge_sites 的 ID 会被 SQLite 复用，而 charge_site_secrets
// 若残留旧站点的密钥，新站点创建时 version=1 插入会撞唯一约束（错误被忽略），
// 回调就会拿旧密钥验签 → 恒报 invalid signature（真机踩到）。
func (r *ChargeSiteRepository) PurgeSiteScopedData(ctx context.Context, siteID int64) error {
	if siteID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{
			"charge_site_secrets",
			"callback_nonces",
			"charge_callback_events",
			"charge_callback_retries",
			"charge_security_events",
		} {
			if !tx.Migrator().HasTable(table) {
				continue
			}
			cols, err := tx.Migrator().ColumnTypes(table)
			if err != nil {
				return err
			}
			hasSiteID := false
			for _, c := range cols {
				if c.Name() == "site_id" {
					hasSiteID = true
					break
				}
			}
			if !hasSiteID {
				continue
			}
			if err := tx.Exec("DELETE FROM "+table+" WHERE site_id = ?", siteID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
