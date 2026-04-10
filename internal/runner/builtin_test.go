package runner

import (
	"regexp"
	"strings"
	"testing"
)

func TestBuiltinNow(t *testing.T) {
	val, ok := evalBuiltin("__now", nil)
	if !ok {
		t.Fatal("evalBuiltin returned !ok for __now")
	}
	// Should be RFC3339 format.
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).MatchString(val) {
		t.Errorf("expected RFC3339, got %q", val)
	}
}

func TestBuiltinTimestamp(t *testing.T) {
	val, ok := evalBuiltin("__timestamp", nil)
	if !ok {
		t.Fatal("not ok")
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(val) {
		t.Errorf("expected integer, got %q", val)
	}
}

func TestBuiltinUUID(t *testing.T) {
	val, ok := evalBuiltin("__uuid", nil)
	if !ok {
		t.Fatal("not ok")
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(val) {
		t.Errorf("expected UUID v4, got %q", val)
	}
}

func TestBuiltinBase64(t *testing.T) {
	vars := map[string]string{"SECRET": "hello"}
	val, ok := evalBuiltin("__base64:SECRET", vars)
	if !ok {
		t.Fatal("not ok")
	}
	if val != "aGVsbG8=" {
		t.Errorf("base64(hello) = %q, want aGVsbG8=", val)
	}
}

func TestBuiltinSHA256(t *testing.T) {
	vars := map[string]string{"MSG": "hello"}
	val, ok := evalBuiltin("__sha256:MSG", vars)
	if !ok {
		t.Fatal("not ok")
	}
	// SHA256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	if val != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Errorf("sha256(hello) = %q", val)
	}
}

func TestBuiltinHMACSHA256(t *testing.T) {
	vars := map[string]string{"KEY": "secret", "MSG": "hello"}
	val, ok := evalBuiltin("__hmac_sha256:KEY:MSG", vars)
	if !ok {
		t.Fatal("not ok")
	}
	// HMAC-SHA256(key=secret, msg=hello) = 88aab3ede8d3adf94d26ab90d3bafd4a2083070c3bcce9c014ee04a443847c0b
	if val != "88aab3ede8d3adf94d26ab90d3bafd4a2083070c3bcce9c014ee04a443847c0b" {
		t.Errorf("hmac_sha256 = %q", val)
	}
}

func TestBuiltinInExpandString(t *testing.T) {
	vars := map[string]string{"USER": "alice"}
	result, err := expandString("user=${USER}&ts=${__timestamp}&id=${__uuid}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "user=alice") {
		t.Errorf("missing USER expansion: %s", result)
	}
	if !regexp.MustCompile(`ts=\d+`).MatchString(result) {
		t.Errorf("missing timestamp expansion: %s", result)
	}
	if !regexp.MustCompile(`id=[0-9a-f]{8}-`).MatchString(result) {
		t.Errorf("missing uuid expansion: %s", result)
	}
}

func TestBuiltinRandomInt(t *testing.T) {
	val, ok := evalBuiltin("__randomInt:100", nil)
	if !ok {
		t.Fatal("not ok")
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(val) {
		t.Errorf("expected integer, got %q", val)
	}
}
