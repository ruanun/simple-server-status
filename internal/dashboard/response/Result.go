package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Result 统一响应结构
type Result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}
