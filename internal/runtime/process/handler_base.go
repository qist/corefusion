package process

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// baseHandler provides common fields and methods for all process handlers.
type baseHandler struct {
	c   *gin.Context
	db  *gorm.DB
	tx  *gorm.DB
	err error
}

// newBaseHandler creates a baseHandler from context and DB.
func newBaseHandler(c *gin.Context, db *gorm.DB) baseHandler {
	return baseHandler{c: c, db: db, tx: db}
}

// jsonError sends a JSON error response and returns true.
func (b *baseHandler) jsonError(code int, msg string) bool {
	b.c.JSON(code, gin.H{"error": msg})
	b.err = fmt.Errorf("%s", msg)
	return true
}

// jsonOK sends a JSON success response with the given data.
func (b *baseHandler) jsonOK(data gin.H) {
	b.c.JSON(http.StatusOK, data)
}

// handleError sends an error response if err is not nil, returns true on error.
func (b *baseHandler) handleError(err error, code int, msg string) bool {
	if err != nil {
		b.jsonError(code, fmt.Sprintf("%s: %v", msg, err))
		return true
	}
	return false
}

// withTransaction runs fn in a transaction. If fn returns error, the transaction is rolled back.
func (b *baseHandler) withTransaction(fn func(tx *gorm.DB) error) error {
	return b.db.Transaction(func(tx *gorm.DB) error {
		b.tx = tx
		return fn(tx)
	})
}
