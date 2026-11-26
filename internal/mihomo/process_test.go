package mihomo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestVerifySHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mihomo.exe")
	data := []byte("pinned mihomo bytes")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])
	if err := VerifySHA256(path, strings.ToUpper(expected)); err != nil {
		t.Fatalf("matching uppercase hash rejected: %v", err)
	}
	if err := VerifySHA256(path, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("mismatched hash error = %v", err)
	}
	if err := VerifySHA256(path, "not-a-hash"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}

func testProcessConfig(t *testing.T) ProcessConfig {
	t.Helper()
	dir := t.TempDir()
	executable := filepath.Join(dir, "mihomo.exe")
	config := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(executable, []byte("must not be launched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("mixed-port: 17897\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("must not be launched"))
	return ProcessConfig{
		MihomoPath:     executable,
		MihomoSHA256:   hex.EncodeToString(sum[:]),
		DataDir:        dir,
		ConfigPath:     config,
		ControllerPort: 19097,
		MixedPort:      17897,
		Secret:         "secret",
	}
}

func TestSupervisorRejectsHashMismatchBeforeStart(t *testing.T) {
	config := testProcessConfig(t)
	config.MihomoSHA256 = strings.Repeat("0", 64)
	supervisor, err := NewSupervisor(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = supervisor.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("start error = %v", err)
	}
	status := supervisor.Status()
	if status.Running || status.PID != 0 {
		t.Fatalf("hash mismatch left a running process: %#v", status)
	}
}

func TestWaitForControllerRetriesWithBearerAuthentication(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer controller-secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if attempts.Add(1) == 1 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":true,"version":"test"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := waitForController(ctx, server.URL, "controller-secret", nil, nil); err != nil {
		t.Fatalf("wait for Controller: %v", err)
	}
	if attempts.Load() < 2 {
		t.Fatalf("Controller was not retried: %d attempts", attempts.Load())
	}
}

func TestWaitForControllerTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := waitForController(ctx, server.URL, "secret", nil, nil)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestWaitForControllerStopsWhenProcessExits(t *testing.T) {
	done := make(chan struct{})
	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitForController(ctx, "http://127.0.0.1:1", "secret", done, func() string {
		return "mihomo exited: exit status 1"
	})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("process exit error = %v", err)
	}
}
