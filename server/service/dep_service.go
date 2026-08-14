package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DepService 提供 Python(pip)与 Node(npm)依赖的列出/安装/卸载。
// 命令一律参数化构造(不经过 shell),包名做字符白名单校验,防注入。
type DepService struct {
	mu sync.Mutex // 安装/卸载串行,避免系统级并发互扰
}

var defaultDepService = &DepService{}

func GetDepService() *DepService { return defaultDepService }

// PkgInfo 依赖包信息。
type PkgInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// opTimeout 安装/卸载超时(网络操作可能较慢)。
const opTimeout = 180 * time.Second

// maxOutput 返回给前端的命令输出上限,避免超大响应。
const maxOutput = 200 * 1024

// pkgNamePattern 允许的包名/版本约束字符(pip 支持 ==、>=、~=、[extras] 等)。
var pkgNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._\-=<>~\[\],+]+$`)

// ---- Python (pip3) ----

// ListPython 列出系统 python3 已安装的包。
func (s *DepService) ListPython() ([]PkgInfo, error) {
	if _, err := exec.LookPath("pip3"); err != nil {
		return nil, fmt.Errorf("未找到 pip3,请确认已安装 Python3")
	}
	out, err := runCmd(context.Background(), 30*time.Second, "pip3", "list", "--format=json")
	if err != nil {
		return nil, fmt.Errorf("执行 pip3 list 失败: %v", err)
	}
	var pkgs []PkgInfo
	if err := json.Unmarshal([]byte(out), &pkgs); err != nil {
		return nil, fmt.Errorf("解析 pip3 list 输出失败: %v", err)
	}
	return pkgs, nil
}

// InstallPython 安装 pip 包。
func (s *DepService) InstallPython(pkg string) (string, error) {
	if err := validatePkg(pkg); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := runCmd(context.Background(), opTimeout, "pip3", "install", pkg)
	if err != nil {
		return out, fmt.Errorf("安装失败: %v", err)
	}
	return out, nil
}

// UninstallPython 卸载 pip 包。
func (s *DepService) UninstallPython(pkg string) (string, error) {
	if err := validatePkg(pkg); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := runCmd(context.Background(), opTimeout, "pip3", "uninstall", "-y", pkg)
	if err != nil {
		return out, fmt.Errorf("卸载失败: %v", err)
	}
	return out, nil
}

// ---- Node (npm 全局) ----

// ListNode 列出 npm 全局包。
func (s *DepService) ListNode() ([]PkgInfo, error) {
	if _, err := exec.LookPath("npm"); err != nil {
		return nil, fmt.Errorf("未找到 npm,请确认已安装 Node.js")
	}
	out, err := runCmd(context.Background(), 30*time.Second, "npm", "list", "-g", "--json", "--depth=0")
	if err != nil {
		return nil, fmt.Errorf("执行 npm list 失败: %v", err)
	}
	var res struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return nil, fmt.Errorf("解析 npm list 输出失败: %v", err)
	}
	var pkgs []PkgInfo
	for name, info := range res.Dependencies {
		pkgs = append(pkgs, PkgInfo{Name: name, Version: info.Version})
	}
	return pkgs, nil
}

// InstallNode 安装 npm 全局包。
func (s *DepService) InstallNode(pkg string) (string, error) {
	if err := validatePkg(pkg); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := runCmd(context.Background(), opTimeout, "npm", "install", "-g", pkg)
	if err != nil {
		return out, fmt.Errorf("安装失败: %v", err)
	}
	return out, nil
}

// UninstallNode 卸载 npm 全局包。
func (s *DepService) UninstallNode(pkg string) (string, error) {
	if err := validatePkg(pkg); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := runCmd(context.Background(), opTimeout, "npm", "uninstall", "-g", pkg)
	if err != nil {
		return out, fmt.Errorf("卸载失败: %v", err)
	}
	return out, nil
}

// ---- 工具 ----

func validatePkg(pkg string) error {
	if len(pkg) > 200 {
		return fmt.Errorf("包名过长")
	}
	// 拒绝任何空白/控制字符,避免 TrimSpace 掩盖非法输入(如 "pkg\n")。
	if strings.ContainsAny(pkg, " \t\n\r\x00") {
		return fmt.Errorf("包名包含非法字符")
	}
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return fmt.Errorf("包名不能为空")
	}
	if !pkgNamePattern.MatchString(pkg) {
		return fmt.Errorf("包名包含非法字符")
	}
	return nil
}

// runCmd 参数化执行命令并合并输出,超时后返回错误。
func runCmd(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if len(out) > maxOutput {
		out = out[:maxOutput] + "\n... [输出已截断]"
	}
	return out, err
}
