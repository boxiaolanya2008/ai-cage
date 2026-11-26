package app

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSecret(t *testing.T) {
	first, err := newSecret()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSecret()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("generated identical secrets")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("secret is not URL-safe base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("secret length = %d, want 32 bytes", len(decoded))
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "runtime.yaml")
	if err := writeFileAtomic(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want second", data)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestValidatePort(t *testing.T) {
	for _, port := range []int{-1, 0, 65536} {
		if err := validatePort("test", port); err == nil {
			t.Fatalf("port %d accepted", port)
		}
	}
	for _, port := range []int{1, 9097, 65535} {
		if err := validatePort("test", port); err != nil {
			t.Fatalf("port %d rejected: %v", port, err)
		}
	}
}

func TestValidDNS(t *testing.T) {
	for _, value := range []string{"223.5.5.5", "119.29.29.29"} {
		if !validDNS(value) {
			t.Fatalf("valid DNS rejected: %s", value)
		}
	}
	for _, value := range []string{"", "localhost", "256.1.1.1", "::1", "1.2.3"} {
		if validDNS(value) {
			t.Fatalf("invalid DNS accepted: %s", value)
		}
	}
}

func TestCalculateMihomoSHA256(t *testing.T) {
	if _, err := (&AppService{}).CalculateMihomoSHA256(strings.Repeat("x", 2)); err == nil {
		t.Fatal("missing file accepted")
	}
}
