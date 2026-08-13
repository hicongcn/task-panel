package service

import "testing"

// TestTokenize 验证命令拆分(支持引号包裹),为命令解析的安全性提供回归保障。
func TestTokenize(t *testing.T) {
	cases := []struct {
		in    string
		want  []string
		valid bool
	}{
		{`python3 demo.py`, []string{"python3", "demo.py"}, true},
		{`bash "my script.sh"`, []string{"bash", "my script.sh"}, true},
		{`node a.js 'arg with space'`, []string{"node", "a.js", "arg with space"}, true},
		{`echo 'unterminated`, nil, false},
	}
	for _, c := range cases {
		got, err := tokenize(c.in)
		if (err == nil) != c.valid {
			t.Errorf("tokenize(%q) valid=%v, want %v (err=%v)", c.in, err == nil, c.valid, err)
			continue
		}
		if !c.valid {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("tokenize(%q) len=%d, want %d (%v)", c.in, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("tokenize(%q)[%d]=%q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
