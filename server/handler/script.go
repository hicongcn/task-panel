package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/pathutil"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

const maxScriptSize = 10 * 1024 * 1024 // 10MB

var allowedScriptExts = map[string]bool{
	".py": true, ".js": true, ".mjs": true, ".ts": true, ".sh": true,
	".go": true, ".json": true, ".yaml": true, ".yml": true, ".txt": true, ".md": true,
}

var scriptInterpByExt = map[string]string{
	".py": "python3", ".js": "node", ".mjs": "node", ".ts": "node", ".sh": "bash", ".go": "go",
}

type ScriptHandler struct{}

func NewScriptHandler() *ScriptHandler { return &ScriptHandler{} }

func scriptsDir() string { return config.C.Data.ScriptsDir }

// Tree GET /scripts/tree
func (h *ScriptHandler) Tree(c *gin.Context) {
	dir := scriptsDir()
	tree := buildTree(dir, "")
	response.Success(c, gin.H{"data": tree})
}

func buildTree(baseDir, prefix string) []map[string]interface{} {
	dir := filepath.Join(baseDir, prefix)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []map[string]interface{}{}
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var out []map[string]interface{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel := e.Name()
		if prefix != "" {
			rel = prefix + "/" + e.Name()
		}
		if e.IsDir() {
			children := buildTree(baseDir, rel)
			out = append(out, map[string]interface{}{
				"key": rel, "title": e.Name(), "type": "directory", "children": children,
			})
		} else {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if !allowedScriptExts[ext] && ext != "" {
				continue
			}
			out = append(out, map[string]interface{}{
				"key": rel, "title": e.Name(), "type": "file", "extension": ext,
			})
		}
	}
	return out
}

// Content GET /scripts/content?path=
func (h *ScriptHandler) Content(c *gin.Context) {
	rel := c.Query("path")
	full, err := pathutil.SafeJoin(scriptsDir(), rel, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if info, err := os.Stat(full); err == nil && info.IsDir() {
		response.BadRequest(c, "目标是目录,不能作为文件打开")
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		response.NotFound(c, "文件不存在")
		return
	}
	response.Success(c, gin.H{"data": gin.H{"path": rel, "content": string(data)}})
}

// Save PUT /scripts/content
func (h *ScriptHandler) Save(c *gin.Context) {
	var req struct {
		Path    string `json:"path" binding:"required"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if len(req.Content) > maxScriptSize {
		response.BadRequest(c, "脚本内容过大")
		return
	}
	full, err := pathutil.SafeJoin(scriptsDir(), req.Path, false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 与上传一致:文件类型白名单,防止写入任意扩展名文件。
	if ext := strings.ToLower(filepath.Ext(full)); ext != "" && !allowedScriptExts[ext] {
		response.BadRequest(c, "不支持的文件类型")
		return
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		response.InternalError(c, "创建目录失败")
		return
	}
	if err := os.WriteFile(full, []byte(req.Content), 0o644); err != nil {
		response.InternalError(c, "保存失败")
		return
	}
	recordAudit(c, model.AuditActionScriptSave, req.Path, "")
	response.Success(c, gin.H{"message": "保存成功"})
}

// CreateDir POST /scripts/directory
func (h *ScriptHandler) CreateDir(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	full, err := pathutil.SafeJoin(scriptsDir(), req.Path, false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		response.InternalError(c, "创建目录失败")
		return
	}
	recordAudit(c, model.AuditActionScriptCreate, req.Path, "")
	response.Created(c, gin.H{"message": "已创建"})
}

// Delete DELETE /scripts?path=
func (h *ScriptHandler) Delete(c *gin.Context) {
	rel := c.Query("path")
	full, err := pathutil.SafeJoin(scriptsDir(), rel, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := os.RemoveAll(full); err != nil {
		response.InternalError(c, "删除失败")
		return
	}
	recordAudit(c, model.AuditActionScriptDelete, rel, "")
	response.Success(c, gin.H{"message": "已删除"})
}

// Rename PUT /scripts/rename
func (h *ScriptHandler) Rename(c *gin.Context) {
	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if strings.ContainsAny(req.NewName, `/\`) || req.NewName == "." || req.NewName == ".." {
		response.BadRequest(c, "新名称非法")
		return
	}
	oldFull, err := pathutil.SafeJoin(scriptsDir(), req.OldPath, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	newFull := filepath.Join(filepath.Dir(oldFull), req.NewName)
	if !pathutil.IsWithinBaseSafe(scriptsDir(), newFull) {
		response.BadRequest(c, "检测到路径穿越")
		return
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		response.InternalError(c, "重命名失败")
		return
	}
	recordAudit(c, model.AuditActionScriptRename, req.OldPath+" -> "+req.NewName, "")
	response.Success(c, gin.H{"message": "已重命名"})
}

// Move PUT /scripts/move {old_path, new_dir} 把文件移动到脚本目录内其他目录。
func (h *ScriptHandler) Move(c *gin.Context) {
	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewDir  string `json:"new_dir" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	base := scriptsDir()
	oldFull, err := pathutil.SafeJoin(base, req.OldPath, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 目标必须是脚本目录内的已存在目录
	newDirFull, err := pathutil.SafeJoin(base, req.NewDir, false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	info, err := os.Stat(newDirFull)
	if err != nil || !info.IsDir() {
		response.BadRequest(c, "目标目录不存在")
		return
	}
	dest := filepath.Join(newDirFull, filepath.Base(oldFull))
	if _, err := os.Stat(dest); err == nil {
		response.BadRequest(c, "目标位置已存在同名文件")
		return
	}
	if err := os.Rename(oldFull, dest); err != nil {
		response.InternalError(c, "移动失败")
		return
	}
	recordAudit(c, model.AuditActionScriptRename, req.OldPath+" -> "+filepath.Join(req.NewDir, filepath.Base(oldFull)), "")
	response.Success(c, gin.H{"message": "已移动"})
}

// Upload POST /scripts/upload  (multipart: file, path=目标目录相对路径)
func (h *ScriptHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择文件")
		return
	}
	if file.Size > maxScriptSize {
		response.BadRequest(c, "文件过大")
		return
	}
	name := filepath.Base(file.Filename)
	if name == "" || name == "." || name == ".." {
		response.BadRequest(c, "文件名无效")
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !allowedScriptExts[ext] {
		response.BadRequest(c, "不支持的文件类型")
		return
	}
	targetDir := strings.TrimSpace(c.PostForm("path"))
	full, err := pathutil.SafeJoin(scriptsDir(), filepath.Join(targetDir, name), false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		response.InternalError(c, "创建目录失败")
		return
	}
	if err := c.SaveUploadedFile(file, full); err != nil {
		response.InternalError(c, "保存失败")
		return
	}
	recordAudit(c, model.AuditActionScriptUpload, name, "")
	response.Created(c, gin.H{"message": "上传成功", "path": relFromScripts(full)})
}

// Download GET /scripts/download?path=
func (h *ScriptHandler) Download(c *gin.Context) {
	rel := c.Query("path")
	full, err := pathutil.SafeJoin(scriptsDir(), rel, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(full, filepath.Base(full))
}

// Run POST /scripts/run  (运行指定脚本路径,同步返回输出)
func (h *ScriptHandler) Run(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	full, err := pathutil.SafeJoin(scriptsDir(), req.Path, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	plan := &service.CommandPlan{
		Interpreter: interpForExt(full), ScriptPath: full,
		WorkDir: filepath.Dir(full),
	}
	env := service.NewEnvService().BuildTaskEnv()
	result := service.RunScriptDebug(plan, env, 60*time.Second)
	recordAudit(c, model.AuditActionScriptRun, req.Path, "")
	response.Success(c, gin.H{"data": gin.H{
		"output": result.Output, "exit_code": result.ExitCode,
		"duration": result.Duration, "timed_out": result.TimedOut,
	}})
}

// RunCode POST /scripts/run-code  (运行内联代码,临时文件)
func (h *ScriptHandler) RunCode(c *gin.Context) {
	var req struct {
		Code     string `json:"code" binding:"required"`
		Language string `json:"language" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	ext, ok := langExt(req.Language)
	if !ok {
		response.BadRequest(c, "不支持的语言")
		return
	}
	tmpDir := filepath.Join(os.TempDir(), "task-panel-debug")
	_ = os.MkdirAll(tmpDir, 0o755)
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("code_%d%s", time.Now().UnixMilli(), ext))
	if err := os.WriteFile(tmpFile, []byte(req.Code), 0o644); err != nil {
		response.InternalError(c, "创建临时文件失败")
		return
	}
	defer os.Remove(tmpFile)

	plan := &service.CommandPlan{
		Interpreter: interpForExt(tmpFile), ScriptPath: tmpFile, WorkDir: tmpDir,
	}
	env := service.NewEnvService().BuildTaskEnv()
	result := service.RunScriptDebug(plan, env, 60*time.Second)
	recordAudit(c, model.AuditActionScriptCode, req.Language, "")
	response.Success(c, gin.H{"data": gin.H{
		"output": result.Output, "exit_code": result.ExitCode,
		"duration": result.Duration, "timed_out": result.TimedOut,
	}})
}

func (h *ScriptHandler) RegisterRoutes(r *gin.RouterGroup) {
	scripts := r.Group("/scripts", middleware.JWTAuth())
	{
		scripts.GET("/tree", h.Tree)
		scripts.GET("/content", h.Content)
		scripts.PUT("/content", h.Save)
		scripts.POST("/directory", h.CreateDir)
		scripts.DELETE("", h.Delete)
		scripts.PUT("/rename", h.Rename)
		scripts.PUT("/move", h.Move)
		scripts.POST("/upload", h.Upload)
		scripts.GET("/download", h.Download)
		scripts.POST("/run", h.Run)
		scripts.POST("/run-code", h.RunCode)
	}
}

// helpers
func interpForExt(path string) string {
	if v, ok := scriptInterpByExt[strings.ToLower(filepath.Ext(path))]; ok {
		return v
	}
	return "bash"
}

func langExt(lang string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "python", "py":
		return ".py", true
	case "javascript", "js":
		return ".js", true
	case "shell", "sh":
		return ".sh", true
	case "go":
		return ".go", true
	}
	return "", false
}

func relFromScripts(full string) string {
	absDir, _ := filepath.Abs(scriptsDir())
	rel, _ := filepath.Rel(absDir, full)
	return filepath.ToSlash(rel)
}
