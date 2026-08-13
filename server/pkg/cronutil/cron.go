// Package cronutil 提供 Cron 表达式校验与中文可读描述。
package cronutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
)

// parser 同时接受标准 5 段表达式与带秒的 6 段表达式。
var parser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// Parser 返回调度器可用的 cron 解析器实例。
func Parser() cron.Parser { return parser }

// Validate 校验 Cron 表达式,合法返回 nil。
func Validate(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return fmt.Errorf("Cron 表达式不能为空")
	}
	if _, err := parser.Parse(strings.TrimSpace(expr)); err != nil {
		return fmt.Errorf("无效的 Cron 表达式 %q: %v", expr, err)
	}
	return nil
}

// Describe 返回表达式的简要中文描述;无法精确描述时返回原表达式。
func Describe(expr string) string {
	expr = strings.TrimSpace(expr)
	if err := Validate(expr); err != nil {
		return expr
	}
	return describeSimple(expr)
}

// describeSimple 只覆盖常见简单形式,复杂表达式原话返回。
func describeSimple(expr string) string {
	fields := strings.Fields(expr)

	// 描述符(@daily 等)字段数不固定,必须在 5/6 段守卫之前处理。
	if len(fields) >= 1 {
		switch strings.ToLower(fields[0]) {
		case "@every":
			return "每 " + strings.Join(fields[1:], " ") + " 执行"
		case "@daily", "@midnight":
			return "每天 00:00 执行"
		case "@hourly":
			return "每小时整点执行"
		case "@weekly":
			return "每周日 00:00 执行"
		case "@monthly":
			return "每月 1 日 00:00 执行"
		case "@yearly", "@annually":
			return "每年 1 月 1 日 00:00 执行"
		}
	}

	offset := 0
	if len(fields) == 6 {
		offset = 1 // 带秒字段
	} else if len(fields) != 5 {
		return expr
	}

	minute := fields[offset]
	hour := fields[offset+1]
	dom := fields[offset+2]
	month := fields[offset+3]
	dow := fields[offset+4]

	if dom == "*" && month == "*" && dow == "*" {
		switch {
		case minute == "*" && hour == "*", minute == "*/1":
			return "每分钟执行"
		case minute == "0" && hour == "*":
			return "每小时整点执行"
		}
	}
	if isSingle(minute) && isSingle(hour) && dom == "*" && month == "*" && dow == "*" {
		m, _ := strconv.Atoi(minute)
		h, _ := strconv.Atoi(hour)
		return fmt.Sprintf("每天 %02d:%02d 执行", h, m)
	}
	if isSingle(minute) && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		m, _ := strconv.Atoi(minute)
		return fmt.Sprintf("每小时的第 %d 分钟执行", m)
	}
	return expr
}

func isSingle(value string) bool {
	if value == "" || value == "*" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}
