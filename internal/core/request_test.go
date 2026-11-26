package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validTestRequest(t *testing.T) Request {
	t.Helper()
	dir := t.TempDir()
	mihomoPath := filepath.Join(dir, "mihomo.exe")
	configPath := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(mihomoPath, []byte("test executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("mixed-port: 17897\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Request{
		MihomoPath:     mihomoPath,
		MihomoSHA256:   strings.Repeat("0", sha256.Size*2),
		DataDir:        dir,
		ConfigPath:     configPath,
		ControllerPort: 19097,
		MixedPort:      17897,
		Secret:         "controller-secret",
	}
}

func TestValidateRequest(t *testing.T) {
	req := validTestRequest(t)
	got, err := validateRequest(req)
	if err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if got.MihomoPath != req.MihomoPath || got.ConfigPath != req.ConfigPath {
		t.Fatalf("paths changed: %#v", got)
	}

	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{"missing executable", func(r *Request) { r.MihomoPath = "" }},
		{"relative executable", func(r *Request) { r.MihomoPath = "mihomo.exe" }},
		{"short hash", func(r *Request) { r.MihomoSHA256 = "abcd" }},
		{"non hexadecimal hash", func(r *Request) { r.MihomoSHA256 = strings.Repeat("z", 64) }},
		{"missing data directory", func(r *Request) { r.DataDir = "" }},
		{"config outside data directory", func(r *Request) { r.ConfigPath = filepath.Join(filepath.Dir(r.DataDir), "outside.yaml") }},
		{"controller port zero", func(r *Request) { r.ControllerPort = 0 }},
		{"controller port too large", func(r *Request) { r.ControllerPort = 65536 }},
		{"mixed port invalid", func(r *Request) { r.MixedPort = -1 }},
		{"ports equal", func(r *Request) { r.MixedPort = r.ControllerPort }},
		{"missing secret", func(r *Request) { r.Secret = "" }},
		{"secret header injection", func(r *Request) { r.Secret = "secret\r\nInjected: value" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := req
			test.mutate(&candidate)
			if _, err := validateRequest(candidate); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestReadAndRemoveRequest(t *testing.T) {
	req := validTestRequest(t)
	path := filepath.Join(t.TempDir(), "request.json")
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readAndRemoveRequest(path)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if got.Secret != req.Secret || got.ControllerPort != req.ControllerPort {
		t.Fatalf("request changed: %#v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("request file still exists: %v", err)
	}
}

func TestReadAndRemoveRequestDeletesMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAndRemoveRequest(path); err == nil {
		t.Fatal("malformed request accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("malformed request file still exists: %v", err)
	}
}

func TestVerifyHashFixture(t *testing.T) {
	// Keep the request fixture digest deterministic and prove the test helper's
	// executable bytes hash correctly without starting it.
	req := validTestRequest(t)
	sum := sha256.Sum256([]byte("test executable"))
	req.MihomoSHA256 = hex.EncodeToString(sum[:])
	if _, err := validateRequest(req); err != nil {
		t.Fatalf("request with real fixture hash rejected: %v", err)
	}
}
