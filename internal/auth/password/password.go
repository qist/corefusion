package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const defaultCost = 12

// MaxBytes 是 bcrypt 能接受的密码上限（72 字节）。超过时 bcrypt 直接报错，
// 调用方**必须**把错误暴露出去，绝不能退化成明文/可逆存储（历史上出过这种 fallback）。
const MaxBytes = 72

// ErrPasswordTooLong 表示密码超过 bcrypt 的 72 字节上限。
// 显式定义它：既避免把底层库的错误泄漏给接口层，也让调用方能把"调用方传错了"和
// "面板故障"区分开（handler 应回 400 而不是 500）。
var ErrPasswordTooLong = errors.New("password exceeds 72 bytes (bcrypt limit)")

// Hash hashes a password using bcrypt.
func Hash(password string) (string, error) {
	if len(password) > MaxBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), defaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Check compares a password with a hash.
func Check(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
