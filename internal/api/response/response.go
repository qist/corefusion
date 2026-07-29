package response

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/model/dto"
)

// OK sends a 200 success response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, dto.APIResponse{
		Data: data,
	})
}

// Created sends a 201 success response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, dto.APIResponse{
		Data: data,
	})
}

// Error sends an error response with the given status code.
func Error(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, dto.APIError{
		Code:      code,
		Message:   message,
		RequestID: c.GetString("request_id"),
	})
}

// BadRequest sends a 400 error response.
func BadRequest(c *gin.Context, code, message string) {
	Error(c, http.StatusBadRequest, code, message)
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *gin.Context, code, message string) {
	Error(c, http.StatusUnauthorized, code, message)
}

// Forbidden sends a 403 error response.
func Forbidden(c *gin.Context, code, message string) {
	Error(c, http.StatusForbidden, code, message)
}

// NotFound sends a 404 error response.
func NotFound(c *gin.Context, code, message string) {
	Error(c, http.StatusNotFound, code, message)
}

// Conflict sends a 409 error response.
func Conflict(c *gin.Context, code, message string) {
	Error(c, http.StatusConflict, code, message)
}

// Unprocessable sends a 422 error response.
func Unprocessable(c *gin.Context, code, message string) {
	Error(c, http.StatusUnprocessableEntity, code, message)
}

// PaymentRequired sends a 402 error response (insufficient balance, need payment).
func PaymentRequired(c *gin.Context, code, message string) {
	Error(c, http.StatusPaymentRequired, code, message)
}

// InternalError sends a 500 error response.
func InternalError(c *gin.Context, code, message string) {
	Error(c, http.StatusInternalServerError, code, message)
}

// Paginated sends a paginated list response.
func Paginated[T any](c *gin.Context, items []T, total int64, limit, offset int) {
	OK(c, dto.Paginated[T]{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// Pagination holds resolved, clamped pagination parameters.
type Pagination struct {
	Page   int
	Limit  int
	Offset int
}

const (
	// DefaultPageSize is used when the client does not specify a page size.
	DefaultPageSize = 20
	// MaxPageSize caps the page size to avoid overloading the database.
	MaxPageSize = 100
)

// ParsePagination reads pagination params from the request and clamps them.
// It accepts either limit+offset or page+page_size. Defaults: page=1,
// limit=DefaultPageSize (20). Limit is capped at MaxPageSize (100).
// This is the single source of truth for pagination and should replace
// ad-hoc helpers such as getLimitOffset.
func ParsePagination(c *gin.Context) Pagination {
	atoi := func(key string, def int) int {
		v := c.Query(key)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return def
		}
		return n
	}

	limit := atoi("limit", 0)
	if limit <= 0 {
		if ps := atoi("page_size", 0); ps > 0 {
			limit = ps
		}
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	offset := atoi("offset", 0)
	if offset < 0 {
		offset = 0
	}
	if offset == 0 {
		page := atoi("page", 1)
		if page < 1 {
			page = 1
		}
		return Pagination{Page: page, Limit: limit, Offset: (page - 1) * limit}
	}
	// offset provided directly: derive page for convenience
	return Pagination{Page: offset/limit + 1, Limit: limit, Offset: offset}
}
