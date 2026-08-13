// Package pathutil 提供路径穿越防护工具。
//
// 设计要点:
//   - 拒绝绝对路径与 UNC/盘符路径;
//   - 拒绝包含 ".." 段或反斜杠分隔符(统一按 "/" 处理)的输入;
//   - 目标路径存在时解析符号链接,校验解析后的真实路径仍位于基础目录内;
//   - 目标路径不存在时,从最近存在的祖先目录解析符号链接,再拼接剩余段,
//     防止"父目录是软链指向外部"的绕过。
package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SafeJoin 把 relPath(必须是相对路径,不包含 .. 段)安全地拼接到 baseDir 下,
// 并返回解析符号链接后仍位于 baseDir 内的绝对路径。
// mustExist=true 时要求目标路径已存在。
func SafeJoin(baseDir, relPath string, mustExist bool) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	relPath = strings.TrimSpace(relPath)
	if baseDir == "" {
		return "", fmt.Errorf("基础目录不能为空")
	}
	if relPath == "" {
		return "", fmt.Errorf("路径不能为空")
	}

	if err := ValidateRelativePath(relPath); err != nil {
		return "", err
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("基础目录无效: %w", err)
	}

	candidate := filepath.Join(baseAbs, filepath.FromSlash(relPath))
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("无效路径: %w", err)
	}

	resolved, err := resolveWithinBase(baseAbs, absCandidate, mustExist)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// ResolveWithinBase 解析 target(可为相对或绝对路径),确保最终真实路径位于 baseDir 内。
// 适用于"路径来自数据库/历史数据,可能已是绝对路径"的场景(如日志文件路径)。
func ResolveWithinBase(baseDir, target string, mustExist bool) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	target = strings.TrimSpace(target)
	if baseDir == "" || target == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("基础目录无效: %w", err)
	}
	candidate := target
	if !filepath.IsAbs(target) {
		candidate = filepath.Join(baseAbs, filepath.FromSlash(target))
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("无效路径: %w", err)
	}
	return resolveWithinBase(baseAbs, absCandidate, mustExist)
}

// IsWithinBaseSafe 判断一个(可能已存在的)绝对路径是否位于 baseDir 内。
// 供"重命名/移动后目标路径校验"等场景使用:先拼好绝对路径再调本校验。
func IsWithinBaseSafe(baseDir, target string) bool {
	baseDir = strings.TrimSpace(baseDir)
	target = strings.TrimSpace(target)
	if baseDir == "" || target == "" {
		return false
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	// base 自身也要解析软链(例如 macOS /var -> /private/var),
	// 否则 base 用未解析路径、target 用解析后路径,比较会误判为越界。
	baseResolved := resolveExistingPath(baseAbs)
	resolved := resolveFromExistingAncestor(absTarget)
	return isWithinBase(baseResolved, resolved)
}

// ValidateRelativePath 校验相对路径的合法性:
// 拒绝空路径、绝对路径、盘符路径、反斜杠分隔以及任何 ".." 段。
func ValidateRelativePath(relPath string) error {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return fmt.Errorf("路径不能为空")
	}

	normalized := strings.ReplaceAll(relPath, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return fmt.Errorf("不允许绝对路径")
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return fmt.Errorf("不允许盘符路径")
	}

	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return fmt.Errorf("不允许路径穿越")
		}
	}
	return nil
}

// resolveWithinBase 解析 candidate(已拼接到 baseAbs 下),确保真实路径仍在 baseAbs 内。
func resolveWithinBase(baseAbs, candidate string, mustExist bool) (string, error) {
	baseAbs = filepath.Clean(baseAbs)
	// base 自身也要解析软链(例如 macOS /tmp -> /private/tmp),
	// 否则 base 用未解析路径、target 用解析后路径,比较会误判为越界。
	baseResolved := resolveExistingPath(baseAbs)

	var resolved string
	if mustExist {
		if _, err := os.Stat(candidate); err != nil {
			return "", err
		}
		resolved = resolveExistingPath(candidate)
	} else {
		resolved = resolveFromExistingAncestor(candidate)
	}

	if !isWithinBase(baseResolved, resolved) {
		return "", fmt.Errorf("检测到路径穿越")
	}
	return resolved, nil
}

// resolveExistingPath 返回路径的规范绝对路径(展开符号链接)。
func resolveExistingPath(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = resolved
	}
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return cleaned
}

// resolveFromExistingAncestor 从最近的已存在祖先开始解析符号链接,再拼接剩余段,
// 以覆盖"父目录是指向基础目录外的软链"这一情况。
func resolveFromExistingAncestor(path string) string {
	current := filepath.Clean(path)
	var segments []string

	for {
		if _, err := os.Stat(current); err == nil {
			resolved := resolveExistingPath(current)
			for i := len(segments) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, segments[i])
			}
			return resolved
		}
		parent := filepath.Dir(current)
		if parent == current {
			return resolveExistingPath(current)
		}
		segments = append(segments, filepath.Base(current))
		current = parent
	}
}

// isWithinBase 判断 target 是否位于 base 目录内(base 本身也算)。
func isWithinBase(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)

	if runtime.GOOS == "windows" {
		base = strings.ToLower(base)
		target = strings.ToLower(target)
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
