package core

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/mihomo"
)

type supervisorAPI interface {
	Start(context.Context) error
	Stop(context.Context) error
	Status() mihomo.SupervisorStatus
}

type apiServer struct {
	supervisor   supervisorAPI
	token        string
	elevated     bool
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

func newAPIServer(supervisor supervisorAPI, token string, elevated bool) *apiServer {
	return &apiServer{
		supervisor: supervisor,
		token:      token,
		elevated:   elevated,
		shutdown:   make(chan struct{}, 1),
	}
}

func (s *apiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/start", s.handleStart)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *apiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	if !s.authorized(w, r) {
		return
	}
	status := s.supervisor.Status()
	writeJSON(w, http.StatusOK, Status{
		Running:      status.Running,
		CoreElevated: s.elevated,
		PID:          status.PID,
		Message:      status.Message,
	})
}

func (s *apiServer) handleStart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !s.authorized(w, r) {
		return
	}
	if err := s.supervisor.Start(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *apiServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !s.authorized(w, r) {
		return
	}
	if err := s.supervisor.Stop(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *apiServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !s.authorized(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.shutdownOnce.Do(func() {
		select {
		case s.shutdown <- struct{}{}:
		default:
		}
	})
}

func (s *apiServer) authorized(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || s.token == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="ai-cage-core"`)
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return false
	}
	provided := []byte(strings.TrimPrefix(header, prefix))
	if subtle.ConstantTimeCompare(provided, []byte(s.token)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="ai-cage-core"`)
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return false
	}
	return true
}

func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	return false
}

func writeError(w http.ResponseWriter, status int, err error) {
	message := "internal core error"
	if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(value); err != nil {
		http.Error(w, "internal core error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buffer.Bytes())
}

// Run consumes the UI request file and serves the supervisor API until a
// shutdown request or process signal is received.
func Run(requestPath string) error {
	req, err := readAndRemoveRequest(requestPath)
	if err != nil {
		return err
	}
	supervisor, err := mihomo.NewSupervisor(mihomo.ProcessConfig{
		MihomoPath:     req.MihomoPath,
		MihomoSHA256:   req.MihomoSHA256,
		DataDir:        req.DataDir,
		ConfigPath:     req.ConfigPath,
		ControllerPort: req.ControllerPort,
		MixedPort:      req.MixedPort,
		Secret:         req.Secret,
	})
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on core loopback API: %w", err)
	}
	defer listener.Close()
	token, err := newToken()
	if err != nil {
		return err
	}
	elevated := isElevated()
	connector := Connector{Endpoint: "http://" + listener.Addr().String(), Token: token, Elevated: elevated}
	if err := writeConnector(connector); err != nil {
		return err
	}
	defer func() { _ = removeConnector(connector) }()

	api := newAPIServer(supervisor, token, elevated)
	server := &http.Server{
		Handler:           api.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	signals, stopSignals := signalContext()
	defer stopSignals()
	var runErr error
	select {
	case <-api.shutdown:
	case <-signals.Done():
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve core API: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		if runErr == nil {
			runErr = fmt.Errorf("shutdown core API: %w", err)
		}
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()
	if err := supervisor.Stop(stopCtx); err != nil && runErr == nil {
		runErr = fmt.Errorf("stop mihomo: %w", err)
	}
	return runErr
}

// Client is an authenticated HTTP client for a running core.
type Client struct {
	connector Connector
	http      *http.Client
}

// NewClient creates a client for connector.
func NewClient(connector Connector) *Client {
	return &Client{
		connector: connector,
		http: &http.Client{
			Timeout: 100 * time.Second,
			Transport: &http.Transport{
				Proxy:                 nil,
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
	}
}

func (c *Client) request(ctx context.Context, method, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.connector.Endpoint, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("create core request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.connector.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("core request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body)
	if body.Error != "" {
		return fmt.Errorf("core %s returned %s: %s", path, resp.Status, body.Error)
	}
	return fmt.Errorf("core %s returned %s", path, resp.Status)
}

// Start asks the core to verify and start mihomo.
func (c *Client) Start(ctx context.Context) error { return c.request(ctx, http.MethodPost, "/start") }

// Stop asks the core to stop mihomo without shutting down the API.
func (c *Client) Stop(ctx context.Context) error { return c.request(ctx, http.MethodPost, "/stop") }

// Status returns the current sidecar state.
func (c *Client) Status(ctx context.Context) (Status, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.connector.Endpoint, "/")+"/status", nil)
	if err != nil {
		return Status{}, fmt.Errorf("create core status request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.connector.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("core status request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return Status{}, fmt.Errorf("core status returned %s", resp.Status)
	}
	var status Status
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&status); err != nil {
		return Status{}, fmt.Errorf("decode core status: %w", err)
	}
	return status, nil
}

// Shutdown asks the core to stop and exit.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/shutdown")
}
