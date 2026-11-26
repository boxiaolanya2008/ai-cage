package subscription

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/config"
	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

const validConfig = "proxies:\n  - name: test\n    type: http\n    server: 127.0.0.1\n    port: 8080\n"

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := config.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	return manager, dir
}

func TestManagerCRUDAndReadConfig(t *testing.T) {
	manager, dir := newTestManager(t)
	if manager.client.Timeout != 20*time.Second {
		t.Fatalf("timeout = %v, want 20s", manager.client.Timeout)
	}
	var version atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "# version %d\n%s", version.Add(1), validConfig)
	}))
	defer server.Close()

	items, err := manager.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("initial List() = %#v, %v", items, err)
	}
	added, err := manager.Add("  Test subscription  ", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if added.Name != "Test subscription" || added.URL != server.URL || added.ID == "" {
		t.Fatalf("unexpected subscription: %#v", added)
	}
	if len(added.ID) != 32 {
		t.Fatalf("ID length = %d, want 32", len(added.ID))
	}
	if _, err := hex.DecodeString(added.ID); err != nil {
		t.Fatalf("ID is not random hex: %v", err)
	}
	path := filepath.Join(dir, subscriptionsDir, added.ID+".yaml")
	data, err := manager.ReadConfig(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("# version 1")) {
		t.Fatalf("unexpected initial config: %q", data)
	}
	if saved, err := os.ReadFile(path); err != nil || !bytes.Equal(saved, data) {
		t.Fatalf("saved YAML mismatch, err=%v", err)
	}

	updated, err := manager.Update(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != added.Name || updated.URL != added.URL || updated.UpdatedAt.Before(added.UpdatedAt) {
		t.Fatalf("unexpected update: %#v", updated)
	}
	data, err = manager.ReadConfig(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("# version 2")) {
		t.Fatalf("update did not replace YAML: %q", data)
	}
	items, err = manager.List()
	if err != nil || len(items) != 1 || items[0].ID != added.ID {
		t.Fatalf("List() = %#v, %v", items, err)
	}

	second, err := manager.Add("Second", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == added.ID {
		t.Fatal("two subscriptions received the same ID")
	}
	if err := manager.Delete(added.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("YAML still exists after delete, err=%v", err)
	}
	if _, err := manager.ReadConfig(added.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadConfig deleted ID error = %v, want ErrNotFound", err)
	}
	if err := manager.Delete(added.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete unknown ID error = %v, want ErrNotFound", err)
	}
	items, err = manager.List()
	if err != nil || len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("List() after delete = %#v, %v", items, err)
	}
}

func TestImportSnapshotReplacesExistingVergeSnapshot(t *testing.T) {
	manager, dir := newTestManager(t)
	first, err := manager.ImportSnapshot("Verge 当前配置", []byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != model.SubscriptionKindVerge || first.URL != "verge://current" {
		t.Fatalf("unexpected snapshot: %#v", first)
	}

	second, err := manager.ImportSnapshot("Verge 新配置", []byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("repeat import created a duplicate: first=%s second=%s", first.ID, second.ID)
	}
	if second.Name != "Verge 新配置" {
		t.Fatalf("snapshot name = %q, want Verge 新配置", second.Name)
	}
	items, err := manager.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("List() = %#v, %v", items, err)
	}
	if _, err := manager.Update(first.ID); err == nil {
		t.Fatal("Update accepted a Verge snapshot")
	}
	data, err := manager.ReadConfig(first.ID)
	if err != nil || !bytes.Contains(data, []byte("proxies:")) {
		t.Fatalf("ReadConfig() = %q, %v", data, err)
	}
	path := filepath.Join(dir, subscriptionsDir, first.ID+".yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
}

func TestImportSnapshotRejectsInvalidData(t *testing.T) {
	manager, _ := newTestManager(t)
	if _, err := manager.ImportSnapshot("Snapshot", []byte("rules: []\n")); err == nil {
		t.Fatal("snapshot without proxies accepted")
	}
	items, err := manager.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("invalid snapshot registered: %#v, %v", items, err)
	}
}

func TestAddRejectsInvalidURLs(t *testing.T) {
	manager, _ := newTestManager(t)
	for _, rawURL := range []string{"", "ftp://example.com/config.yaml", "file:///tmp/config.yaml", "http:///missing-host", "://bad"} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := manager.Add("Test", rawURL); err == nil {
				t.Fatalf("Add accepted URL %q", rawURL)
			}
		})
	}
	if _, err := manager.Add(" ", "https://example.com/config.yaml"); err == nil {
		t.Fatal("Add accepted an empty name")
	}
}

func TestAddRejectsInvalidYAML(t *testing.T) {
	cases := map[string]string{
		"malformed":       "proxies: [",
		"missing fields":  "port: 7890\nmixed-port: 7891\n",
		"null fields":     "proxies: null\n",
		"empty proxies":   "proxies: []\n",
		"empty providers": "proxy-providers: {}\n",
		"multiple docs":   "proxies: []\n---\nproxies: []\n",
		"top-level array": "- proxies\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			manager, dir := newTestManager(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			if _, err := manager.Add("Test", server.URL); err == nil {
				t.Fatal("Add accepted invalid YAML")
			}
			entries, err := os.ReadDir(filepath.Join(dir, subscriptionsDir))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid YAML left files: %v", entries)
			}
		})
	}
}

func TestAddAcceptsProxyProviders(t *testing.T) {
	manager, _ := newTestManager(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxy-providers:\n  example:\n    type: http\n    url: https://example.com/provider\n"))
	}))
	defer server.Close()
	if _, err := manager.Add("Provider", server.URL); err != nil {
		t.Fatalf("proxy-providers config rejected: %v", err)
	}
}

func TestAddEnforcesSizeLimit(t *testing.T) {
	for _, declared := range []bool{true, false} {
		t.Run(fmt.Sprintf("declared=%v", declared), func(t *testing.T) {
			manager, dir := newTestManager(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if declared {
					w.Header().Set("Content-Length", fmt.Sprint(MaxContentSize+1))
				}
				_, _ = w.Write(bytes.Repeat([]byte("x"), MaxContentSize+1))
			}))
			defer server.Close()
			if _, err := manager.Add("Too large", server.URL); err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("size error = %v, want exceeds", err)
			}
			entries, err := os.ReadDir(filepath.Join(dir, subscriptionsDir))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("oversized response left files: %v", entries)
			}
		})
	}
}

func TestRedirectsAreLimitedAndValidated(t *testing.T) {
	t.Run("limit", func(t *testing.T) {
		manager, _ := newTestManager(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/loop", http.StatusFound)
		}))
		defer server.Close()
		if _, err := manager.Add("Loop", server.URL+"/loop"); err == nil || !strings.Contains(err.Error(), "redirects") {
			t.Fatalf("redirect error = %v", err)
		}
	})
	t.Run("scheme", func(t *testing.T) {
		manager, _ := newTestManager(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "file:///tmp/config.yaml", http.StatusFound)
		}))
		defer server.Close()
		if _, err := manager.Add("Bad redirect", server.URL); err == nil || !strings.Contains(err.Error(), "invalid redirect URL") {
			t.Fatalf("redirect error = %v", err)
		}
	})
}

func TestDownloadTimeoutAndHTTPError(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		manager, _ := newTestManager(t)
		manager.client.Timeout = 50 * time.Millisecond
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			_, _ = w.Write([]byte(validConfig))
		}))
		defer server.Close()
		if _, err := manager.Add("Slow", server.URL); err == nil {
			t.Fatal("slow download did not time out")
		}
	})
	t.Run("status", func(t *testing.T) {
		manager, _ := newTestManager(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "no", http.StatusNotFound)
		}))
		defer server.Close()
		if _, err := manager.Add("Missing", server.URL); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("HTTP error = %v", err)
		}
	})
}

func TestDamagedIndexAndInvalidIDs(t *testing.T) {
	manager, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, indexFilename), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.List(); err == nil {
		t.Fatal("List accepted a damaged index")
	}
	for _, id := range []string{"", "../escape", "not-a-valid-id"} {
		if _, err := manager.ReadConfig(id); err == nil {
			t.Fatalf("ReadConfig accepted ID %q", id)
		}
		if err := manager.Delete(id); err == nil {
			t.Fatalf("Delete accepted ID %q", id)
		}
	}
}

func TestManagerRejectsNilStore(t *testing.T) {
	if _, err := NewManager(nil); err == nil {
		t.Fatal("NewManager accepted a nil store")
	}
	var manager *Manager
	if _, err := manager.List(); err == nil {
		t.Fatal("nil manager List did not fail")
	}
}
