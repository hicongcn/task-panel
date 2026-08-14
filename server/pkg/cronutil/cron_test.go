package cronutil

import "testing"

func TestValidate(t *testing.T) {
	cases := []struct {
		expr  string
		valid bool
	}{
		{"*/5 * * * *", true},
		{"0 2 * * *", true},
		{"0 */6 * * *", true},
		{"@daily", true},
		{"0 0 1 1 *", true},
		{"", false},
		{"not a cron", false},
		{"* * * * * *", true}, // 带秒
	}
	for _, c := range cases {
		err := Validate(c.expr)
		if (err == nil) != c.valid {
			t.Errorf("Validate(%q) valid=%v, want %v (err=%v)", c.expr, err == nil, c.valid, err)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]string{
		"* * * * *":      "每分钟执行",
		"0 * * * *":      "每小时整点执行",
		"0 3 * * *":      "每天 03:00 执行",
		"30 8 * * *":     "每天 08:30 执行",
		"*/1 * * * *":    "每分钟执行",
		"@daily":         "每天 00:00 执行",
	}
	for expr, want := range cases {
		got := Describe(expr)
		if got != want {
			t.Errorf("Describe(%q) = %q, want %q", expr, got, want)
		}
	}
}

func TestDescribeEnhanced(t *testing.T) {
	cases := map[string]string{
		"*/5 * * * *":    "每 5 分钟执行",
		"*/10 * * * * *": "每 10 秒执行",
		"* * * * * *":    "每秒执行",
		"0 */2 * * *":    "每 2 小时执行",
		"0 * * * *":      "每小时整点执行",
		"30 8 * * *":     "每天 08:30 执行",
		"0 8 * * 1":      "每周一 08:00 执行",
		"0 8 1 * *":      "每月 1 日 08:00 执行",
		"15 * * * *":     "每小时的第 15 分钟执行",
	}
	for expr, want := range cases {
		if got := Describe(expr); got != want {
			t.Errorf("Describe(%q) = %q, want %q", expr, got, want)
		}
	}
}
