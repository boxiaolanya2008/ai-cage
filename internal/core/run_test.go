package core

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunPublishesConnectorAndConsumesRequest(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	req := validTestRequest(t)
	requestPath := filepath.Join(t.TempDir(), "request.json")
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(requestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	runDone := make(chan error, 1)
	go func() { runDone <- Run(requestPath) }()
	var connector Connector
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		connector, err = ReadConnector()
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("core did not publish connector: %v", err)
	}
	parsed, err := url.Parse(connector.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Hostname() != "127.0.0.1" {
		t.Fatalf("core bound non-loopback endpoint: %s", connector.Endpoint)
	}
	if connector.Token == "" {
		t.Fatal("core published an empty token")
	}
	if _, err := os.Stat(requestPath); !os.IsNotExist(err) {
		t.Fatalf("core did not consume request: %v", err)
	}

	client := NewClient(connector)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("query core status: %v", err)
	}
	if status.Running || status.CoreElevated != connector.Elevated {
		t.Fatalf("unexpected initial status: %#v", status)
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown core: %v", err)
	}
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("core did not exit after shutdown")
	}
	if _, err := ReadConnector(); err == nil {
		t.Fatal("core connector remains after shutdown")
	}
}
