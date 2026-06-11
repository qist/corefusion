package users

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/service/user"
)

// Handler handles user HTTP requests.
type Handler struct {
	userSvc *user.Service
}

// NewHandler creates a new user handler.
func NewHandler(userSvc *user.Service) *Handler {
	return &Handler{userSvc: userSvc}
}

// List handles GET /api/v1/users.
func (h *Handler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	users, total, err := h.userSvc.List(c.Request.Context(), q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_USERS_FAILED", err.Error())
		return
	}

	// Strip password hashes
	items := make([]gin.H, len(users))
	for i, u := range users {
		items[i] = gin.H{
			"id":         u.ID,
			"username":   u.Username,
			"email":      u.Email,
			"role":       u.Role,
			"status":     u.Status,
			"created_at": u.CreatedAt,
			"updated_at": u.UpdatedAt,
		}
	}

	response.Paginated(c, items, total, q.Limit, q.Offset)
}

// Create handles POST /api/v1/users.
func (h *Handler) Create(c *gin.Context) {
	var input user.CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	u, err := h.userSvc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_USER_FAILED", err.Error())
		return
	}

	response.Created(c, gin.H{
		"id":         u.ID,
		"username":   u.Username,
		"email":      u.Email,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt,
	})
}

// Update handles PUT /api/v1/users/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid user id")
		return
	}

	var input user.UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	u, err := h.userSvc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_USER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":         u.ID,
		"username":   u.Username,
		"email":      u.Email,
		"role":       u.Role,
		"status":     u.Status,
		"updated_at": u.UpdatedAt,
	})
}

// Delete handles DELETE /api/v1/users/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid user id")
		return
	}

	if err := h.userSvc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_USER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "user deleted"})
}
