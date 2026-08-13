package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"taskpanel/config"
	"taskpanel/pkg/pathutil"
)

// CommandPlan 是解析后的任务执行计划。
// 所有字段都直接作为 exec.Command 的参数,绝不经过 shell,杜绝注入。
type CommandPlan struct {
	Interpreter string   // 解释器二进制名:python3/node/bash/go
	ScriptPath  string   // 脚本绝对路径(已校验位于脚本目录内)
	Args        []string // 额外参数
	WorkDir     string   // 工作目录(脚本所在目录)
}

var extInterpreterMap = map[string]string{
	".py":  "python3",
	".js":  "node",
	".mjs": "node",
	".ts":  "node", // MVP 不单独处理 ts-node,走 node(用户自行配 loader)
	".sh":  "bash",
	".go":  "go",
}

// ParseCommand 解析任务命令字符串为执行计划。
// 支持两种写法:
//   1. <解释器> <脚本相对路径> [args...]  例: python3 mytask.py --flag
//   2. <脚本相对路径> [args...]           例: mytask.py --flag (按扩展名推断解释器)
// 脚本路径必须在配置的脚本目录内,且不允许穿越。
func ParseCommand(command string) (*CommandPlan, error) {
	tokens, err := tokenize(command)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("命令不能为空")
	}

	scriptsDir := config.C.Data.ScriptsDir
	first := tokens[0]
	rest := tokens[1:]

	// 情况 1:显式解释器
	if interp, ok := knownInterpreter(first); ok {
		if len(rest) == 0 {
			return nil, fmt.Errorf("缺少脚本路径")
		}
		scriptPath, err := resolveScript(rest[0], scriptsDir)
		if err != nil {
			return nil, err
		}
		return &CommandPlan{
			Interpreter: interp,
			ScriptPath:  scriptPath,
			Args:        append([]string{}, rest[1:]...),
			WorkDir:     filepath.Dir(scriptPath),
		}, nil
	}

	// 情况 2:按脚本扩展名推断解释器
	scriptPath, err := resolveScript(first, scriptsDir)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(scriptPath))
	interp, ok := extInterpreterMap[ext]
	if !ok {
		return nil, fmt.Errorf("不支持的脚本扩展名: %s", ext)
	}
	return &CommandPlan{
		Interpreter: interp,
		ScriptPath:  scriptPath,
		Args:        append([]string{}, rest...),
		WorkDir:     filepath.Dir(scriptPath),
	}, nil
}

// knownInterpreter 判断 name 是否为受支持的解释器,返回规范名。
func knownInterpreter(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "python", "python3":
		return "python3", true
	case "node":
		return "node", true
	case "bash", "sh":
		return "bash", true
	case "go":
		return "go", true
	}
	return "", false
}

// resolveScript 把脚本相对路径解析到脚本目录内,并校验存在性与穿越。
func resolveScript(relPath, scriptsDir string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("脚本路径不能为空")
	}
	full, err := pathutil.SafeJoin(scriptsDir, relPath, true)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		return "", fmt.Errorf("脚本不存在或不是文件: %s", relPath)
	}
	return full, nil
}

// tokenize 把命令字符串按空白拆分,支持单/双引号包裹的整段参数。
func tokenize(command string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for _, r := range command {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			cur.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			flush()
			continue
		}
		cur.WriteRune(r)
	}
	if quote != 0 {
		return nil, fmt.Errorf("命令引号未闭合")
	}
	flush()
	return tokens, nil
}

// CommandParts 返回解释器 + 全部参数(exec.Command 直接使用)。
func (p *CommandPlan) CommandParts() (string, []string) {
	args := append([]string{}, p.Args...)
	if p.Interpreter == "go" {
		// go run <script> [args]
		return "go", append([]string{"run", p.ScriptPath}, args...)
	}
	return p.Interpreter, append([]string{p.ScriptPath}, args...)
}
