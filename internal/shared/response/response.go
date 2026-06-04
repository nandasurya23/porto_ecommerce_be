package response

import "github.com/gin-gonic/gin"

type ErrorItem struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, gin.H{"success": true, "message": message, "data": data})
}

func SuccessWithMeta(c *gin.Context, status int, message string, data any, meta any) {
	c.JSON(status, gin.H{"success": true, "message": message, "data": data, "meta": meta})
}

func Error(c *gin.Context, status int, message string, errors []ErrorItem) {
	payload := gin.H{"success": false, "message": message}
	if len(errors) > 0 {
		payload["errors"] = errors
	}
	c.JSON(status, payload)
}
