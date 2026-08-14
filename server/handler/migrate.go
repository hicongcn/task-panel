package handler

import (
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type MigrateHandler struct {
	svc *service.MigrateService
}

func NewMigrateHandler() *MigrateHandler { return &MigrateHandler{svc: service.NewMigrateService()} }

// Export GET /migrate/export 导出全部任务/脚本/环境变量。
func (h *MigrateHandler) Export(c *gin.Context) {
	data, err := h.svc.Export()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionMigrateExport, "", "")
	response.Success(c, gin.H{"data": data})
}

// Import POST /migrate/import body 为导出的 JSON 结构。
func (h *MigrateHandler) Import(c *gin.Context) {
	var req service.ExportData
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "导入文件格式错误")
		return
	}
	if len(req.Tasks) == 0 && len(req.Scripts) == 0 && len(req.Envs) == 0 {
		response.BadRequest(c, "导入内容为空")
		return
	}
	res, err := h.svc.Import(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionMigrateImport, "", "")
	response.Success(c, gin.H{"message": "导入完成", "data": res})
}

func (h *MigrateHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/migrate", middleware.JWTAuth())
	{
		g.GET("/export", h.Export)
		g.POST("/import", h.Import)
	}
}
