package middleware

import (
	"errors"
	"net/http"

	appErrors "footwear-backend/internal/shared/errors"
	"footwear-backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		last := c.Errors.Last().Err
		var appErr *appErrors.AppError
		if errors.As(last, &appErr) {
			response.Error(c, appErr.Status, appErr.Message, nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
	}
}
