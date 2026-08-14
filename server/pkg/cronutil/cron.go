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

// describeSimple 覆盖常见简单形式,复杂表达式原话返回。
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
	sec := "*"
	if len(fields) == 6 {
		offset = 1
		sec = fields[0] // 带秒字段
	} else if len(fields) != 5 {
		return expr
	}

	minute := fields[offset]
	hour := fields[offset+1]
	dom := fields[offset+2]
	month := fields[offset+3]
	dow := fields[offset+4]

	allStar := dom == "*" && month == "*" && dow == "*"

	// 秒级:每秒 / 每 N 秒(6 段表达式)
	if offset == 1 && minute == "*" && allStar {
		switch {
		case sec == "*":
			return "每秒执行"
		case isStep(sec):
			n := stepNum(sec)
			if n == 1 {
				return "每秒执行"
			}
			return fmt.Sprintf("每 %d 秒执行", n)
		}
	}
	// 每分钟(分钟、小时均为 *)
	if minute == "*" && hour == "*" && allStar {
		return "每分钟执行"
	}
	// 每小时整点(分钟 0,小时任意)
	if minute == "0" && hour == "*" && allStar {
		return "每小时整点执行"
	}
	// 分钟级:每 N 分钟(如 */5 * * * *)
	if isStep(minute) && hour == "*" && allStar {
		n := stepNum(minute)
		if n == 1 {
			return "每分钟执行"
		}
		return fmt.Sprintf("每 %d 分钟执行", n)
	}
	// 小时级:每 N 小时(如 0 */2 * * *)
	if minute == "0" && isStep(hour) && allStar {
		n := stepNum(hour)
		if n == 1 {
			return "每小时整点执行"
		}
		return fmt.Sprintf("每 %d 小时执行", n)
	}
	// 每天 HH:MM
	if isSingle(minute) && isSingle(hour) && allStar {
		m, _ := strconv.Atoi(minute)
		h, _ := strconv.Atoi(hour)
		return fmt.Sprintf("每天 %02d:%02d 执行", h, m)
	}
	// 每周几 HH:MM(如 0 8 * * 1)
	if isSingle(minute) && isSingle(hour) && dom == "*" && month == "*" && isSingle(dow) {
		m, _ := strconv.Atoi(minute)
		h, _ := strconv.Atoi(hour)
		d, _ := strconv.Atoi(dow)
		return fmt.Sprintf("每周%s %02d:%02d 执行", weekdayCN(d), h, m)
	}
	// 每月几号 HH:MM(如 0 8 1 * *)
	if isSingle(minute) && isSingle(hour) && isSingle(dom) && month == "*" && dow == "*" {
		m, _ := strconv.Atoi(minute)
		h, _ := strconv.Atoi(hour)
		d, _ := strconv.Atoi(dom)
		return fmt.Sprintf("每月 %d 日 %02d:%02d 执行", d, h, m)
	}
	// 每小时的第 N 分钟
	if isSingle(minute) && hour == "*" && allStar {
		m, _ := strconv.Atoi(minute)
		return fmt.Sprintf("每小时的第 %d 分钟执行", m)
	}
	return expr
}

// weekdayCN 数字星期(0=周日,7=周日)转中文。
func weekdayCN(d int) string {
	names := []string{"日", "一", "二", "三", "四", "五", "六"}
	if d < 0 || d > 7 {
		return fmt.Sprintf("%d", d)
	}
	if d == 7 {
		d = 0
	}
	return names[d]
}

func isSingle(value string) bool {
	if value == "" || value == "*" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

// isStep 判断是否为 */N 形式。
func isStep(value string) bool {
	if !strings.HasPrefix(value, "*/") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(value, "*/"))
	return err == nil && n > 0
}

func stepNum(value string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(value, "*/"))
	return n
}
