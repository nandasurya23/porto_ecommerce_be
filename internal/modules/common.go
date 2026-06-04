package modules

import (
	"net/http"
	"strings"

	"footwear-backend/internal/middleware"
	appErrors "footwear-backend/internal/shared/errors"
	"footwear-backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type userContext struct {
	ID    string
	Email string
	Role  string
}

func currentUser(c *gin.Context) userContext {
	id, _ := c.Get(middleware.ContextUserID)
	email, _ := c.Get(middleware.ContextEmail)
	role, _ := c.Get(middleware.ContextRole)
	return userContext{
		ID:    toString(id),
		Email: toString(email),
		Role:  toString(role),
	}
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func mustUUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid id", []response.ErrorItem{{Field: name, Message: "must be a valid UUID"}})
		return uuid.Nil, false
	}
	return id, true
}

func badRequest(c *gin.Context, message string, field string) {
	response.Error(c, http.StatusBadRequest, message, []response.ErrorItem{{Field: field, Message: message}})
}

func unauthorized(c *gin.Context) {
	response.Error(c, http.StatusUnauthorized, "Unauthorized", nil)
}

func forbidden(c *gin.Context) {
	response.Error(c, http.StatusForbidden, "You do not have permission", nil)
}

func notFound(c *gin.Context, message string) {
	response.Error(c, http.StatusNotFound, message, nil)
}

func appError(status int, code, message string) error {
	return appErrors.New(status, code, message)
}

func normalizeSearch(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func uuidPtrToString(v *uuid.UUID) *string {
	if v == nil {
		return nil
	}
	s := v.String()
	return &s
}
