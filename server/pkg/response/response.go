// Package response 提供统一的 HTTP 响应封装。
// 所有 API 均返回 { code, message, data } 结构,code=0 表示成功。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回 200 + 业务成功结构。
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, body{Code: 0, Message: "ok", Data: data})
}

// Created 返回 201 + 业务成功结构。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, body{Code: 0, Message: "ok", Data: data})
}

// Error 返回指定 HTTP 状态码 + 业务错误结构。
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, body{Code: status, Message: message})
}

// BadRequest 返回 400。
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized 返回 401。
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden 返回 403。
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound 返回 404。
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalError 返回 500。
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}

// TooManyRequests 返回 429。
func TooManyRequests(c *gin.Context, message string) {
	Error(c, http.StatusTooManyRequests, message)
}
