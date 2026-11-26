package mihomo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProcessConfig contains everything needed to start an independent mihomo.
type ProcessConfig struct {
	MihomoPath     string
	MihomoSHA256   string
	DataDir        string
	ConfigPath     string
	ControllerPort int
	MixedPort      int
	Secret         string
}

// SupervisorStatus is a point-in-time sidecar status.
type SupervisorStatus struct {
	Running bool
	PID     int
	Message string
}

type processJob struct {
	once     sync.Once
	closeErr error
	close    func() error
	assign   func(*os.Process) error
}

// Supervisor owns one mihomo process and, on Windows, its Job Object.
type Supervisor struct {
	config ProcessConfig

	opMu    sync.Mutex
	mu      sync.RWMutex
	cmd     *exec.Cmd
	done    chan struct{}
	job     *processJob
	state   string
	message string
	waitErr error
}

// NewSupervisor validates the process configuration without starting mihomo.
func NewSupervisor(config ProcessConfig) (*Supervisor, error) {
	var err error
	config.MihomoPath, err = filepath.Abs(strings.TrimSpace(config.MihomoPath))
	if err != nil {
		return nil, fmt.Errorf("resolve mihomo path: %w", err)
	}
	config.DataDir, err = filepath.Abs(strings.TrimSpace(config.DataDir))
	if err != nil {
		return nil, fmt.Errorf("resolve mihomo data directory: %w", err)
	}
	config.ConfigPath, err = filepath.Abs(strings.TrimSpace(config.ConfigPath))
	if err != nil {
		return nil, fmt.Errorf("resolve mihomo config path: %w", err)
	}
	config.MihomoSHA256 = strings.ToLower(strings.TrimSpace(config.MihomoSHA256))
	if len(config.MihomoSHA256) != sha256.Size*2 {
		return nil, errors.New("invalid mihomo SHA-256")
	}
	if _, err := hex.DecodeString(config.MihomoSHA256); err != nil {
		return nil, errors.New("invalid mihomo SHA-256")
	}
	if !validPort(config.ControllerPort) || !validPort(config.MixedPort) || config.ControllerPort == config.MixedPort {
		return nil, errors.New("invalid mihomo controller or mixed port")
	}
	if strings.TrimSpace(config.Secret) == "" || strings.ContainsAny(config.Secret, "\r\n") {
		return nil, errors.New("invalid mihomo controller secret")
	}
	rel, err := filepath.Rel(config.DataDir, config.ConfigPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("mihomo config must be inside the data directory")
	}
	return &Supervisor{config: config, state: "stopped", message: "stopped"}, nil
}

// VerifySHA256 verifies path against a pinned hexadecimal SHA-256 digest.
func VerifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != sha256.Size*2 {
		return errors.New("invalid expected SHA-256")
	}
	want, err := hex.DecodeString(expected)
	if err != nil {
		return errors.New("invalid expected SHA-256")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open mihomo for verification: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat mihomo for verification: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("mihomo path is not a regular file")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash mihomo: %w", err)
	}
	got := hash.Sum(nil)
	for i := range want {
		if want[i] != got[i] {
			return errors.New("mihomo SHA-256 mismatch")
		}
	}
	return nil
}

// Start verifies mihomo, starts it with fixed arguments and waits for /version.
func (s *Supervisor) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.cleanupFinishedLocked()
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	if state == "running" || state == "starting" || state == "stopping" {
		return errors.New("mihomo is already running")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := VerifySHA256(s.config.MihomoPath, s.config.MihomoSHA256); err != nil {
		s.setFailure(err)
		return err
	}
	if info, err := os.Stat(s.config.DataDir); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		s.setFailure(fmt.Errorf("mihomo data directory: %w", err))
		return fmt.Errorf("mihomo data directory: %w", err)
	}

	job, err := createProcessJob()
	if err != nil {
		s.setFailure(err)
		return err
	}
	cmd := exec.Command(s.config.MihomoPath, "-d", s.config.DataDir, "-f", s.config.ConfigPath)
	cmd.Dir = s.config.DataDir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureProcess(cmd)
	s.mu.Lock()
	s.state = "starting"
	s.message = "starting mihomo"
	s.mu.Unlock()
	if err := cmd.Start(); err != nil {
		_ = closeProcessJob(job)
		s.setFailure(fmt.Errorf("start mihomo: %w", err))
		return fmt.Errorf("start mihomo: %w", err)
	}
	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.done = done
	s.job = job
	s.mu.Unlock()
	go s.wait(cmd, done)
	if err := assignProcessToJob(job, cmd.Process); err != nil {
		_ = s.stopLocked(context.Background())
		wrapped := fmt.Errorf("assign mihomo to job: %w", err)
		s.setFailure(wrapped)
		return wrapped
	}
	if err := waitForController(ctx, fmt.Sprintf("http://127.0.0.1:%d", s.config.ControllerPort), s.config.Secret, done, s.processExitMessage); err != nil {
		_ = s.stopLocked(context.Background())
		s.setFailure(err)
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = s.stopLocked(context.Background())
		s.setFailure(err)
		return err
	}
	s.mu.Lock()
	if s.cmd == cmd && s.state == "starting" {
		s.state = "running"
		s.message = "running"
	}
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) wait(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	s.mu.Lock()
	var job *processJob
	if s.cmd == cmd {
		job = s.job
		s.waitErr = err
		s.state = "stopped"
		if err == nil {
			s.message = "mihomo exited"
		} else {
			s.message = "mihomo exited: " + err.Error()
		}
	}
	s.mu.Unlock()
	if job != nil {
		_ = closeProcessJob(job)
	}
	close(done)
}

// Stop first requests graceful termination, then force-kills the process tree.
func (s *Supervisor) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.stopLocked(ctx)
}

func (s *Supervisor) stopLocked(ctx context.Context) error {
	s.mu.RLock()
	cmd, done, job := s.cmd, s.done, s.job
	s.mu.RUnlock()
	if cmd == nil || done == nil {
		return nil
	}
	s.mu.Lock()
	if s.state != "stopped" {
		s.state = "stopping"
		s.message = "stopping mihomo"
	}
	s.mu.Unlock()

	if !channelClosed(done) {
		graceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = gracefulStop(graceCtx, cmd.Process)
		select {
		case <-done:
		case <-graceCtx.Done():
		}
		cancel()
	}
	var forceErr error
	if !channelClosed(done) {
		forceErr = forceStop(job, cmd.Process)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			s.mu.Lock()
			s.message = "mihomo did not exit after forced termination"
			s.mu.Unlock()
			return errors.Join(forceErr, errors.New("mihomo did not exit after forced termination"))
		}
	}
	if job != nil {
		if err := closeProcessJob(job); err != nil && forceErr == nil {
			forceErr = err
		}
	}
	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
		s.done = nil
		s.job = nil
		s.waitErr = nil
		s.state = "stopped"
		s.message = "stopped"
	}
	s.mu.Unlock()
	if forceErr != nil && !channelClosed(done) {
		return forceErr
	}
	return nil
}

// Status reports whether mihomo is fully ready.
func (s *Supervisor) Status() SupervisorStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := SupervisorStatus{Running: s.state == "running", Message: s.message}
	if s.cmd != nil && s.cmd.Process != nil && (s.state == "starting" || s.state == "running" || s.state == "stopping") {
		status.PID = s.cmd.Process.Pid
	}
	return status
}

func (s *Supervisor) cleanupFinishedLocked() {
	s.mu.RLock()
	done, job := s.done, s.job
	s.mu.RUnlock()
	if done != nil && channelClosed(done) {
		if job != nil {
			_ = closeProcessJob(job)
		}
		s.mu.Lock()
		s.cmd = nil
		s.done = nil
		s.job = nil
		s.waitErr = nil
		s.state = "stopped"
		s.mu.Unlock()
	}
}

func (s *Supervisor) setFailure(err error) {
	s.mu.Lock()
	s.state = "stopped"
	s.message = err.Error()
	s.mu.Unlock()
}

func (s *Supervisor) processExitMessage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.waitErr == nil {
		return "mihomo exited"
	}
	return "mihomo exited: " + s.waitErr.Error()
}

func channelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func waitForController(ctx context.Context, endpoint, secret string, done <-chan struct{}, exitMessage func() string) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: time.Second,
		},
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait for mihomo Controller: %w", err)
		}
		select {
		case <-done:
			message := "mihomo exited"
			if exitMessage != nil {
				message = exitMessage()
			}
			return errors.New(message)
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/version", nil)
		if err != nil {
			return fmt.Errorf("create mihomo readiness request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+secret)
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("Controller returned %s", resp.Status)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for mihomo Controller: %w (last error: %v)", ctx.Err(), lastErr)
		case <-done:
			message := "mihomo exited"
			if exitMessage != nil {
				message = exitMessage()
			}
			return errors.New(message)
		case <-ticker.C:
		}
	}
}
