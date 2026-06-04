package middleware

import (
	"net/http"
	"strings"

	"footwear-backend/internal/shared/response"
	"footwear-backend/internal/shared/security"
	"github.com/gin-gonic/gin"
)

const (
	ContextUserID = "user_id"
	ContextEmail  = "user_email"
	ContextRole   = "user_role"
)

func Auth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "Unauthorized", nil)
			c.Abort()
			return
		}
		claims, err := security.ParseJWT(secret, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "Unauthorized", nil)
			c.Abort()
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextEmail, claims.Email)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}
