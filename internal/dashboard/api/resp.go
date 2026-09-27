package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respond 输出成功响应 {"data": ...}
func respond(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// fail 输出错误响应 {"error": {...}} 并中止后续处理
func fail(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": apiError{Code: code, Message: msg}})
}

// internal 记录错误日志并返回 500
func (a *API) internal(c *gin.Context, msg string, err error) {
	a.Log.Error(msg, "err", err)
	fail(c, http.StatusInternalServerError, "internal", msg)
}
