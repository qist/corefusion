package response

import (
	"net/http"

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
