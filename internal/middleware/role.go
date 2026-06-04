package middleware

import (
	"net/http"

	"footwear-backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func RequireRole(allowed ...string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}
	return func(c *gin.Context) {
		role, _ := c.Get(ContextRole)
		r, _ := role.(string)
		if _, ok := allowedSet[r]; !ok {
			response.Error(c, http.StatusForbidden, "You do not have permission", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
