package totp

import (
	"encoding/base32"
	"strings"
	"testing"
)

// TestHotpRFC4226 用 RFC 4226 附录 D 的测试向量校验 HOTP 核心。
func TestHotpRFC4226(t *testing.T) {
	// RFC 4226 key = ASCII "12345678901234567890"
	key := []byte("12345678901234567890")
	cases := []struct {
		counter uint64
		want    string
	}{
		{0, "755224"},
		{1, "287082"},
		{2, "359152"},
		{3, "969429"},
		{4, "338314"},
		{5, "254676"},
		{6, "287922"},
		{7, "162583"},
		{8, "399871"},
		{9, "520489"},
	}
	for _, c := range cases {
		if got := hotp(key, c.counter); got != c.want {
			t.Errorf("counter %d: got %s, want %s", c.counter, got, c.want)
		}
	}
}

// TestHotpRFC6238 用 RFC 6238 附录 B 的 TOTP 测试向量(6 位)。
func TestHotpRFC6238(t *testing.T) {
	// RFC 6238 附录 B 的 secret 即 ASCII "12345678901234567890"
	key := []byte("12345678901234567890")
	cases := []struct {
		timeSec int64
		want    string
	}{
		{59, "287082"},            // T=1
		{1111111109, "081804"},    // T=37037036
		{1111111111, "050471"},    // T=37037037
		{1234567890, "005924"},    // T=41152263
		{2000000000, "279037"},    // T=66666666
		{20000000000, "353130"},   // T=666666666
	}
	for _, c := range cases {
		if got := hotp(key, uint64(c.timeSec/30)); got != c.want {
			t.Errorf("time %d: got %s, want %s", c.timeSec, got, c.want)
		}
	}
}

func TestGenerateSecretAndValidate(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	// secret 应为 base32 且可解码为 20 字节
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil || len(key) != 20 {
		t.Fatalf("secret 非法: %s (%v)", secret, err)
	}

	code := CurrentCode(secret)
	if len(code) != 6 {
		t.Fatalf("CurrentCode 长度应为 6: %s", code)
	}
	if !Validate(secret, code) {
		t.Fatal("当前码应验证通过")
	}
	if Validate(secret, "000000") {
		t.Fatal("错误码不应通过")
	}
	if Validate(secret, "") {
		t.Fatal("空码不应通过")
	}
}

func TestProvisioningURI(t *testing.T) {
	uri := ProvisioningURI("admin", "SECRET")
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("URI 前缀错误: %s", uri)
	}
	if !strings.Contains(uri, "issuer=task-panel") {
		t.Fatalf("URI 缺少 issuer: %s", uri)
	}
	if !strings.Contains(uri, "secret=SECRET") {
		t.Fatalf("URI 缺少 secret: %s", uri)
	}
}
