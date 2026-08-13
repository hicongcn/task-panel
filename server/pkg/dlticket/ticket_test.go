package dlticket

import (
	"testing"
	"time"
)

const secret = "test-secret-key"

func TestIssueVerify(t *testing.T) {
	ticket, expires, err := Issue(secret, "log:42", "admin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if expires.IsZero() {
		t.Error("expiresAt zero")
	}
	user, err := Verify(secret, ticket, "log:42")
	if err != nil {
		t.Errorf("verify failed: %v", err)
	}
	if user != "admin" {
		t.Errorf("user = %q, want admin", user)
	}
}

func TestVerifyTamper(t *testing.T) {
	ticket, _, _ := Issue(secret, "log:1", "u", time.Minute)
	// 篡改票据:把 body 部分换掉,保留原签名
	dot := -1
	for i, c := range ticket {
		if c == '.' { dot = i; break }
	}
	if dot < 0 {
		t.Fatal("ticket has no dot")
	}
	tampered := "AAAAAAAAAAA." + ticket[dot+1:]
	if _, err := Verify(secret, tampered, "log:1"); err == nil {
		t.Error("tampered ticket should be rejected")
	}
}

func TestVerifyWrongResource(t *testing.T) {
	ticket, _, _ := Issue(secret, "log:1", "u", time.Minute)
	if _, err := Verify(secret, ticket, "log:2"); err == nil {
		t.Error("wrong resource should be rejected")
	}
}

func TestVerifyExpired(t *testing.T) {
	// Issue 把 ttl<=0 钳成默认值,所以用 1ms 正 ttl + 等待来触发真实过期路径。
	ticket, _, _ := Issue(secret, "log:1", "u", time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, err := Verify(secret, ticket, "log:1"); err == nil {
		t.Error("expired ticket should be rejected")
	}
}
