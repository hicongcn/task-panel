package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := bytes.Repeat([]byte{0xAB}, 32)
	plain := []byte("hello task-panel backup")
	enc, err := Encrypt(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatal("roundtrip 不一致")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{0xAB}, 32)
	wrong := bytes.Repeat([]byte{0xCD}, 32)
	enc, _ := Encrypt([]byte("data"), key)
	if _, err := Decrypt(enc, wrong); err == nil {
		t.Fatal("错误密钥应解密失败")
	}
}

func TestDecryptBadMagic(t *testing.T) {
	key := bytes.Repeat([]byte{0xAB}, 32)
	if _, err := Decrypt([]byte("not-a-backup"), key); err == nil {
		t.Fatal("非法文件应报错")
	}
}

func TestTarGzUntarRoundtrip(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "scripts")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "a.sh"), []byte("echo a"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "db.sqlite"), []byte("sqlite-data"), 0o644)

	sources := map[string]string{
		"taskpanel.db": filepath.Join(dir, "db.sqlite"),
		"scripts":      sub,
	}
	tgz, err := TarGz(sources)
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if err := UntarGz(tgz, out); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(out, "taskpanel.db")); string(data) != "sqlite-data" {
		t.Fatal("db 未正确恢复")
	}
	if data, _ := os.ReadFile(filepath.Join(out, "scripts", "a.sh")); string(data) != "echo a" {
		t.Fatal("脚本未正确恢复")
	}
}

func TestCreateExtractBackup(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "db.sqlite"), []byte("data"), 0o644)
	sources := map[string]string{"taskpanel.db": filepath.Join(dir, "db.sqlite")}
	dest := filepath.Join(dir, "b.backup")
	if err := CreateBackup(dest, key, sources); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := ExtractBackup(dest, key, out); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(out, "taskpanel.db")); string(data) != "data" {
		t.Fatal("恢复数据不一致")
	}
}

func TestUntarPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "../evil.sh", Mode: 0o644, Size: 4})
	_, _ = tw.Write([]byte("bad\n"))
	_ = tw.Close()
	_ = gw.Close()

	if err := UntarGz(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}
