package validator

import "testing"

func TestValidateUsername(t *testing.T) {
	cases := map[string]bool{
		"admin": true, "user_1": true, "管理员": true, "": false,
		"a b": false, "user-1": false, "user!": false,
	}
	for in, want := range cases {
		if got := ValidateUsername(in); got != want {
			t.Errorf("ValidateUsername(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if !ValidatePassword("123456") {
		t.Error("6-char password should be valid")
	}
	if ValidatePassword("12345") {
		t.Error("5-char password should be invalid")
	}
	if ValidatePassword("") {
		t.Error("empty password should be invalid")
	}
}

func TestValidateEnvName(t *testing.T) {
	cases := map[string]bool{
		"MY_VAR": true, "_OK": true, "VAR1": true,
		"": false, "1VAR": false, "VA-R": false, "VA R": false,
	}
	for in, want := range cases {
		if got := ValidateEnvName(in); got != want {
			t.Errorf("ValidateEnvName(%q) = %v, want %v", in, got, want)
		}
	}
}
