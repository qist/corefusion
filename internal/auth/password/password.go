package password

import (
	"golang.org/x/crypto/bcrypt"
)

const defaultCost = 12

// Hash hashes a password using bcrypt.
func Hash(password string) (string, error) {
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
