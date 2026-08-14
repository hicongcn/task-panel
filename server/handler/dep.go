package handler

import (
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type DepHandler struct {
	svc *service.DepService
}

func NewDepHandler() *DepHandler { return &DepHandler{svc: service.GetDepService()} }

// ListPython GET /deps/python
func (h *DepHandler) ListPython(c *gin.Context) {
	pkgs, err := h.svc.ListPython()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": pkgs})
}

// InstallPython POST /deps/python/install {package}
func (h *DepHandler) InstallPython(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	out, err := h.svc.InstallPython(req.Package)
	if err != nil {
		recordAudit(c, model.AuditActionDepInstall, "pip", req.Package+" (失败)")
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionDepInstall, "pip", req.Package)
	response.Success(c, gin.H{"message": "安装完成", "output": out})
}

// UninstallPython POST /deps/python/uninstall {package}
func (h *DepHandler) UninstallPython(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	out, err := h.svc.UninstallPython(req.Package)
	if err != nil {
		recordAudit(c, model.AuditActionDepUninstall, "pip", req.Package+" (失败)")
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionDepUninstall, "pip", req.Package)
	response.Success(c, gin.H{"message": "卸载完成", "output": out})
}

// ListNode GET /deps/node
func (h *DepHandler) ListNode(c *gin.Context) {
	pkgs, err := h.svc.ListNode()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": pkgs})
}

// InstallNode POST /deps/node/install {package}
func (h *DepHandler) InstallNode(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	out, err := h.svc.InstallNode(req.Package)
	if err != nil {
		recordAudit(c, model.AuditActionDepInstall, "npm", req.Package+" (失败)")
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionDepInstall, "npm", req.Package)
	response.Success(c, gin.H{"message": "安装完成", "output": out})
}

// UninstallNode POST /deps/node/uninstall {package}
func (h *DepHandler) UninstallNode(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	out, err := h.svc.UninstallNode(req.Package)
	if err != nil {
		recordAudit(c, model.AuditActionDepUninstall, "npm", req.Package+" (失败)")
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionDepUninstall, "npm", req.Package)
	response.Success(c, gin.H{"message": "卸载完成", "output": out})
}

func (h *DepHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/deps", middleware.JWTAuth())
	g.GET("/python", h.ListPython)
	g.POST("/python/install", h.InstallPython)
	g.POST("/python/uninstall", h.UninstallPython)
	g.GET("/node", h.ListNode)
	g.POST("/node/install", h.InstallNode)
	g.POST("/node/uninstall", h.UninstallNode)
}
