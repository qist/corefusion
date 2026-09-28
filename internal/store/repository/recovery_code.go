package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// RecoveryCodeRepository handles 2FA recovery codes database operations.
type RecoveryCodeRepository struct {
	db *gorm.DB
}

// NewRecoveryCodeRepository creates a new RecoveryCodeRepository.
func NewRecoveryCodeRepository(db *gorm.DB) *RecoveryCodeRepository {
	return &RecoveryCodeRepository{db: db}
}

// ReplaceByUser 在事务内清除该用户旧恢复码并写入新的一批（重新生成场景）。
func (r *RecoveryCodeRepository) ReplaceByUser(ctx context.Context, userID int64, hashes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&entity.RecoveryCode{}).Error; err != nil {
			return err
		}
		if len(hashes) == 0 {
			return nil
		}
		now := time.Now()
		rows := make([]entity.RecoveryCode, 0, len(hashes))
		for _, h := range hashes {
			rows = append(rows, entity.RecoveryCode{
				UserID:    userID,
				CodeHash:  h,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		return tx.Create(&rows).Error
	})
}

// DeleteByUser 删除该用户全部恢复码（关闭 2FA / 用户删除时调用）。
func (r *RecoveryCodeRepository) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&entity.RecoveryCode{}).Error
}

// CountUnused 返回用户当前未使用的恢复码数量（状态展示用）。
func (r *RecoveryCodeRepository) CountUnused(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.RecoveryCode{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Count(&n).Error
	return n, err
}

// FindUnusedByHash 返回该用户一条未使用且 hash 匹配的恢复码。
func (r *RecoveryCodeRepository) FindUnusedByHash(ctx context.Context, userID int64, codeHash string) (*entity.RecoveryCode, error) {
	var code entity.RecoveryCode
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, codeHash).
		First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// MarkUsed 把恢复码标记为已使用（一次性）。
func (r *RecoveryCodeRepository) MarkUsed(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.RecoveryCode{}).
		Where("id = ?", id).
		UpdateColumn("used_at", &now).Error
}
