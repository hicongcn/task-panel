package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRelativePath(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"a/b.py", true},
		{"demo.py", true},
		{"", false},
		{"/etc/passwd", false},
		{"../secret", false},
		{"a/../../secret", false},
		{"a\\b", true}, // 反斜杠被统一成 / 处理,不报错但合并
		{"C:/x", false},
		{"./ok", true},
	}
	for _, c := range cases {
		err := ValidateRelativePath(c.in)
		if (err == nil) != c.valid {
			t.Errorf("ValidateRelativePath(%q) valid=%v, want %v (err=%v)", c.in, err == nil, c.valid, err)
		}
	}
}

func TestSafeJoinTraversalBlocked(t *testing.T) {
	base := t.TempDir()
	if _, err := os.Create(filepath.Join(base, "file.txt")); err != nil {
		t.Fatal(err)
	}

	// .. 穿越
	if _, err := SafeJoin(base, "../outside", false); err == nil {
		t.Error("SafeJoin should reject .. traversal")
	}
	// 绝对路径
	if _, err := SafeJoin(base, "/etc/passwd", false); err == nil {
		t.Error("SafeJoin should reject absolute path")
	}
	// 软链逃逸:在 base 内建一个指向外部的软链
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outside, []byte("x"), 0o644)
	link := filepath.Join(base, "escape.link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := SafeJoin(base, "escape.link", true); err == nil {
		t.Error("SafeJoin should reject symlink escaping base")
	}
	// 合法路径
	full, err := SafeJoin(base, "file.txt", true)
	if err != nil {
		t.Errorf("legit path rejected: %v", err)
	}
	if _, err := os.Stat(full); err != nil {
		t.Errorf("resolved path not accessible: %v", err)
	}
}

// TestIsWithinBaseSafeSymlink 回归:base 目录自身是软链(macOS /var -> /private/var)
// 时,IsWithinBaseSafe 曾因未解析 base 软链而误判目标越界。
func TestIsWithinBaseSafeSymlink(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// 目标文件尚不存在(重命名场景:校验"未来的"路径)
	target := filepath.Join(sub, "world.py")
	if !IsWithinBaseSafe(base, target) {
		t.Errorf("IsWithinBaseSafe(%q, %q) = false, want true", base, target)
	}
	// 越界目标应拒绝
	if IsWithinBaseSafe(base, filepath.Join(base, "..", "outside.py")) {
		t.Error("IsWithinBaseSafe should reject path outside base")
	}
}
