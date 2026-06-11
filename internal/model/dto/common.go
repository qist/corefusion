package dto

// APIResponse is the standard API response envelope.
type APIResponse struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// APIError is the standard error response.
type APIError struct {
	Code                string `json:"code"`
	Message             string `json:"message"`
	RequestID           string `json:"request_id"`
	CanRetry            bool   `json:"can_retry"`
	RequiresAction      bool   `json:"requires_action"`
	RequiresActionReason string `json:"requires_action_reason,omitempty"`
	Detail              any    `json:"detail,omitempty"`
}

// Paginated is a paginated list response.
type Paginated[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// ListQuery is the common list query parameters.
type ListQuery struct {
	Limit  int    `form:"limit,default=20"`
	Offset int    `form:"offset,default=0"`
	Keyword string `form:"keyword"`
}
