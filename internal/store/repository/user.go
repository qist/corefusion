package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// UserRepository handles user database operations.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user.
func (r *UserRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID retrieves a user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername retrieves a user by username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// List retrieves all users with pagination.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64

	r.db.WithContext(ctx).Model(&entity.User{}).Count(&total)
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

// Update updates a user.
func (r *UserRepository) Update(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete deletes a user by ID.
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.User{}, id).Error
}

// GetTokenVersion 读取用户的当前令牌版本（供鉴权中间件校验）。
func (r *UserRepository) GetTokenVersion(ctx context.Context, userID int64) (int, error) {
	var v int
	err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ?", userID).
		Select("token_version").
		Scan(&v).Error
	if err != nil {
		return 0, err
	}
	return v, nil
}

// UpdatePasswordHash 只更新口令哈希，不触碰其它字段。
//
// 用 UpdateColumn 而非 Save：Save 会把内存里的**旧** token_version 一并写回，
// 可能覆盖掉期间别人 bump 出的新版本 —— 那会让已吊销的 token 重新生效 ✗。
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, userID int64, hash string) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ?", userID).
		UpdateColumn("password_hash", hash).Error
}

// BumpTokenVersion 把用户的令牌版本 +1 —— 这是"立即吊销该用户所有已签发 token"的手段
// （登出 / 踢下线时调用）。用 SQL 自增避免并发覆盖。
func (r *UserRepository) BumpTokenVersion(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

// SetWebAuthnHandle 写入（或懒生成后写入）用户的 WebAuthn User Handle。
// 用 UpdateColumn 而非 Save：避免把内存里的旧 token_version 写回覆盖并发 bump 的版本。
// ⚠ 列名是 GORM 驼峰命名策略生成的 web_authn_handle（WebAuthn → web_authn）。
func (r *UserRepository) SetWebAuthnHandle(ctx context.Context, userID int64, handle string) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).
		Where("id = ?", userID).
		UpdateColumn("web_authn_handle", handle).Error
}

// GetUserByWebAuthnHandle 按 WebAuthn User Handle 反查用户（Passkey discoverable 登录用）。
func (r *UserRepository) GetUserByWebAuthnHandle(ctx context.Context, handle string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("web_authn_handle = ?", handle).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
