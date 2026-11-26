package mihomo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"github.com/coder/websocket"
)

func TestNewClientValidatesLoopbackBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"http://127.0.0.1:9097",
		"http://localhost:9097/",
		"https://[::1]:9097",
		"http://127.1.2.3",
	} {
		t.Run("accept "+baseURL, func(t *testing.T) {
			client, err := NewClient(baseURL, "secret")
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if client.httpClient.Timeout != DefaultTimeout {
				t.Errorf("timeout = %v, want %v", client.httpClient.Timeout, DefaultTimeout)
			}
		})
	}

	for _, baseURL := range []string{
		"",
		"127.0.0.1:9097",
		"ftp://127.0.0.1:9097",
		"http://example.com:9097",
		"http://127.0.0.1.example.com:9097",
		"http://user:pass@127.0.0.1:9097",
		"http://127.0.0.1:9097/api",
		"http://127.0.0.1:9097?token=value",
		"http://127.0.0.1:9097#fragment",
		"http://127.0.0.1:0",
		"http://127.0.0.1:65536",
	} {
		t.Run("reject "+baseURL, func(t *testing.T) {
			if _, err := NewClient(baseURL, "secret"); err == nil {
				t.Fatal("NewClient() error = nil, want an error")
			}
		})
	}
}

func TestClientHTTPMethodsAndAuthentication(t *testing.T) {
	const secret = "test-secret"
	group := "代理 / A?#%"
	selected := "香港 / 01"
	testURL := "https://example.com/a?x=1&y=two words"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBearer(t, r, secret)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/version":
			writeJSON(t, w, model.Version{Meta: true, Version: "v1.19.0"})
		case r.Method == http.MethodGet && r.URL.Path == "/configs":
			writeJSON(t, w, map[string]any{
				"mixed-port":          17897,
				"mode":                "rule",
				"log-level":           "info",
				"allow-lan":           false,
				"tun":                 map[string]bool{"enable": true},
				"external-controller": "127.0.0.1:19097",
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/configs":
			var body struct {
				Mode model.ProxyMode `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode mode request: %v", err)
			}
			if body.Mode != model.ProxyModeGlobal {
				t.Errorf("mode = %q, want %q", body.Mode, model.ProxyModeGlobal)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/proxies":
			writeJSON(t, w, model.Proxies{Proxies: map[string]model.Proxy{
				group: {Name: group, Type: "Select", Now: selected, All: []string{selected}, Alive: true},
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/proxies/"+group:
			if got, want := r.URL.EscapedPath(), "/proxies/"+url.PathEscape(group); got != want {
				t.Errorf("escaped path = %q, want %q", got, want)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode selection request: %v", err)
			}
			if body["name"] != selected {
				t.Errorf("selected proxy = %q, want %q", body["name"], selected)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/connections":
			writeJSON(t, w, model.Connections{
				DownloadTotal: 100,
				UploadTotal:   50,
				Memory:        1024,
				Connections: []model.Connection{{
					ID: "connection-1", Chains: []string{group}, Rule: "Match",
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/proxies/"+group+"/delay":
			if got, want := r.URL.EscapedPath(), "/proxies/"+url.PathEscape(group)+"/delay"; got != want {
				t.Errorf("delay escaped path = %q, want %q", got, want)
			}
			if got := r.URL.Query().Get("url"); got != testURL {
				t.Errorf("delay URL = %q, want %q", got, testURL)
			}
			if got := r.URL.Query().Get("timeout"); got != "3000" {
				t.Errorf("delay timeout = %q, want 3000", got)
			}
			writeJSON(t, w, model.DelayResult{Delay: 42})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, secret)
	ctx := context.Background()

	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version.Version != "v1.19.0" || !version.Meta {
		t.Errorf("Version() = %#v", version)
	}

	config, err := client.Config(ctx)
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}
	if config.MixedPort != 17897 || config.Mode != "rule" || !config.TUN.Enabled {
		t.Errorf("Config() = %#v", config)
	}

	if err := client.SetProxyMode(ctx, model.ProxyModeGlobal); err != nil {
		t.Fatalf("SetProxyMode() with 204 error = %v", err)
	}

	proxies, err := client.Proxies(ctx)
	if err != nil {
		t.Fatalf("Proxies() error = %v", err)
	}
	if got := proxies.Proxies[group]; got.Name != group || got.Now != selected {
		t.Errorf("Proxies()[group] = %#v", got)
	}

	if err := client.SelectProxy(ctx, group, selected); err != nil {
		t.Fatalf("SelectProxy() with 204 error = %v", err)
	}

	connections, err := client.Connections(ctx)
	if err != nil {
		t.Fatalf("Connections() error = %v", err)
	}
	if connections.DownloadTotal != 100 || connections.UploadTotal != 50 || len(connections.Connections) != 1 {
		t.Errorf("Connections() = %#v", connections)
	}

	delay, err := client.Delay(ctx, testURL, group, 3000)
	if err != nil {
		t.Fatalf("Delay() error = %v", err)
	}
	if delay.Delay != 42 {
		t.Errorf("Delay() = %#v, want 42 ms", delay)
	}
}

func TestClientReturnsJSONAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBearer(t, r, "secret")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"invalid secret"}`)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "secret").Version(context.Background())
	if err == nil {
		t.Fatal("Version() error = nil, want an API error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "invalid secret" {
		t.Errorf("APIError = %#v", apiErr)
	}
	if !strings.Contains(err.Error(), "invalid secret") {
		t.Errorf("error %q does not contain API message", err)
	}
}

func TestClientOmitsAuthorizationWhenSecretIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		writeJSON(t, w, model.Version{Version: "test"})
	}))
	defer server.Close()

	if _, err := newTestClient(t, server.URL, "").Version(context.Background()); err != nil {
		t.Fatalf("Version() error = %v", err)
	}
}

func TestClientHonorsRequestTimeoutAndContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "secret")
	client.httpClient.Timeout = 25 * time.Millisecond
	if _, err := client.Version(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Version() timeout error = %v, want context deadline exceeded", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Version(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Version() canceled error = %v, want context canceled", err)
	}
}

func TestClientTrafficWebSocketAndNormalClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBearer(t, r, "secret")
		if r.URL.Path != "/traffic" {
			t.Errorf("path = %q, want /traffic", r.URL.Path)
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept() error = %v", err)
			return
		}
		defer conn.CloseNow()
		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"up":10,"down":20,"upTotal":30,"downTotal":40}`)); err != nil {
			t.Errorf("write traffic: %v", err)
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	traffic, streamErrors, stop, err := newTestClient(t, server.URL, "secret").Traffic(ctx)
	if err != nil {
		t.Fatalf("Traffic() error = %v", err)
	}
	defer stop()

	select {
	case sample := <-traffic:
		if sample.Up != 10 || sample.Down != 20 || sample.UpTotal != 30 || sample.DownTotal != 40 {
			t.Errorf("traffic sample = %#v", sample)
		}
	case err := <-streamErrors:
		t.Fatalf("traffic stream error = %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for traffic sample")
	}
	requireClosed(t, traffic)
	requireClosed(t, streamErrors)
}

func TestClientTrafficWebSocketCancellation(t *testing.T) {
	for _, useReturnedCancel := range []bool{true, false} {
		t.Run(fmt.Sprintf("returned cancel=%t", useReturnedCancel), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Errorf("websocket.Accept() error = %v", err)
					return
				}
				defer conn.CloseNow()
				<-r.Context().Done()
			}))
			defer server.Close()

			ctx, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			traffic, streamErrors, stop, err := newTestClient(t, server.URL, "secret").Traffic(ctx)
			if err != nil {
				t.Fatalf("Traffic() error = %v", err)
			}
			defer stop()
			if useReturnedCancel {
				stop()
			} else {
				cancelParent()
			}

			requireClosed(t, traffic)
			requireClosed(t, streamErrors)
		})
	}
}

func TestClientLogsWebSocket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBearer(t, r, "secret")
		if r.URL.Path != "/logs" || r.URL.Query().Get("level") != "debug" {
			t.Errorf("request URI = %q, want /logs?level=debug", r.URL.RequestURI())
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept() error = %v", err)
			return
		}
		defer conn.CloseNow()
		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"info","payload":"listening"}`)); err != nil {
			t.Errorf("write log: %v", err)
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	logs, streamErrors, stop, err := newTestClient(t, server.URL, "secret").Logs(context.Background(), "debug")
	if err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	defer stop()
	select {
	case message := <-logs:
		if message != "listening" {
			t.Errorf("log message = %q, want listening", message)
		}
	case err := <-streamErrors:
		t.Fatalf("log stream error = %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for log message")
	}
	requireClosed(t, logs)
	requireClosed(t, streamErrors)
}

func TestClientWebSocketReportsDecodeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept() error = %v", err)
			return
		}
		defer conn.CloseNow()
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`not-json`))
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	_, streamErrors, stop, err := newTestClient(t, server.URL, "secret").Traffic(context.Background())
	if err != nil {
		t.Fatalf("Traffic() error = %v", err)
	}
	defer stop()
	select {
	case err := <-streamErrors:
		if err == nil || !strings.Contains(err.Error(), "decode mihomo websocket message") {
			t.Errorf("stream error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for decode error")
	}
	requireClosed(t, streamErrors)
}

func newTestClient(t *testing.T, baseURL, secret string) *Client {
	t.Helper()
	client, err := NewClient(baseURL, secret)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func assertBearer(t *testing.T, r *http.Request, secret string) {
	t.Helper()
	if got, want := r.Header.Get("Authorization"), "Bearer "+secret; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func requireClosed[T any](t *testing.T, channel <-chan T) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-channel:
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for stream channel to close")
		}
	}
}
