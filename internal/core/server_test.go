package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/mihomo"
)

type fakeSupervisor struct {
	mu       sync.Mutex
	status   mihomo.SupervisorStatus
	startErr error
	stopErr  error
	starts   int
	stops    int
}

func (f *fakeSupervisor) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.startErr != nil {
		return f.startErr
	}
	f.status = mihomo.SupervisorStatus{Running: true, PID: 4321, Message: "running"}
	return nil
}

func (f *fakeSupervisor) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	if f.stopErr != nil {
		return f.stopErr
	}
	f.status = mihomo.SupervisorStatus{Message: "stopped"}
	return nil
}

func (f *fakeSupervisor) Status() mihomo.SupervisorStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func TestAPIAuthenticationAndLifecycle(t *testing.T) {
	supervisor := &fakeSupervisor{status: mihomo.SupervisorStatus{Message: "stopped"}}
	api := newAPIServer(supervisor, "correct-token", true)
	server := httptest.NewServer(api.handler())
	defer server.Close()
	client := NewClient(Connector{Endpoint: server.URL, Token: "correct-token", Elevated: true})

	response, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health returned %s", response.Status)
	}

	for _, path := range []string{"/status", "/start", "/stop", "/shutdown"} {
		method := http.MethodGet
		if path != "/status" {
			method = http.MethodPost
		}
		for _, token := range []string{"", "wrong-token"} {
			req, err := http.NewRequest(method, server.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s with token %q returned %s", path, token, response.Status)
			}
		}
	}
	supervisor.mu.Lock()
	if supervisor.starts != 0 || supervisor.stops != 0 {
		t.Fatalf("unauthorized requests reached supervisor: starts=%d stops=%d", supervisor.starts, supervisor.stops)
	}
	supervisor.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("authenticated start: %v", err)
	}
	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("authenticated status: %v", err)
	}
	if !status.Running || !status.CoreElevated || status.PID != 4321 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := client.Stop(ctx); err != nil {
		t.Fatalf("authenticated stop: %v", err)
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("authenticated shutdown: %v", err)
	}
	select {
	case <-api.shutdown:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not notify Run")
	}
}

func TestAPIMethodAndErrorHandling(t *testing.T) {
	supervisor := &fakeSupervisor{startErr: errors.New("hash mismatch")}
	api := newAPIServer(supervisor, "token", false)
	server := httptest.NewServer(api.handler())
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/start", nil)
	req.Header.Set("Authorization", "Bearer token")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed || response.Header.Get("Allow") != http.MethodPost {
		t.Fatalf("unexpected method response: %s Allow=%q", response.Status, response.Header.Get("Allow"))
	}

	req, _ = http.NewRequest(http.MethodPost, server.URL+"/start", nil)
	req.Header.Set("Authorization", "Bearer token")
	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("start error returned %s", response.Status)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] == "" {
		t.Fatal("start error response has no message")
	}
}

func TestClientStatusRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing bearer token")
		}
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	client := NewClient(Connector{Endpoint: server.URL, Token: "token"})
	if _, err := client.Status(context.Background()); err == nil {
		t.Fatal("invalid status JSON accepted")
	}
}
