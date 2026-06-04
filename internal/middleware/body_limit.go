package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func BodyLimit(maxMB int64) gin.HandlerFunc {
	if maxMB <= 0 {
		maxMB = 2
	}
	maxBytes := maxMB * 1024 * 1024
	headerValue := strconv.FormatInt(maxBytes, 10)

	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Max-Body-Size-Bytes", headerValue)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
