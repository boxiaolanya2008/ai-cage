// Package core implements the loopback-only supervisor API used by the UI.
package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Request is the startup configuration written by the UI. Secret is the
// mihomo Controller secret; it is never included in a process command line.
type Request struct {
	MihomoPath     string `json:"mihomoPath"`
	MihomoSHA256   string `json:"mihomoSha256"`
	DataDir        string `json:"dataDir"`
	ConfigPath     string `json:"configPath"`
	ControllerPort int    `json:"controllerPort"`
	MixedPort      int    `json:"mixedPort"`
	Secret         string `json:"secret"`
}

// Connector describes the authenticated loopback API of a running core.
type Connector struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
	Elevated bool   `json:"elevated"`
}

// Status is the current sidecar state reported by the core API.
type Status struct {
	Running      bool   `json:"running"`
	CoreElevated bool   `json:"coreElevated"`
	PID          int    `json:"pid"`
	Message      string `json:"message"`
}

func validateRequest(req Request) (Request, error) {
	var errs []string
	req.MihomoPath = strings.TrimSpace(req.MihomoPath)
	req.MihomoSHA256 = strings.ToLower(strings.TrimSpace(req.MihomoSHA256))
	req.DataDir = strings.TrimSpace(req.DataDir)
	req.ConfigPath = strings.TrimSpace(req.ConfigPath)
	req.Secret = strings.TrimSpace(req.Secret)

	if req.MihomoPath == "" {
		errs = append(errs, "mihomo path is required")
	} else if !filepath.IsAbs(req.MihomoPath) {
		errs = append(errs, "mihomo path must be absolute")
	}
	if len(req.MihomoSHA256) != 64 {
		errs = append(errs, "mihomo SHA-256 must contain 64 hexadecimal characters")
	} else {
		for _, r := range req.MihomoSHA256 {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
				errs = append(errs, "mihomo SHA-256 must contain 64 hexadecimal characters")
				break
			}
		}
	}
	if req.DataDir == "" || !filepath.IsAbs(req.DataDir) {
		errs = append(errs, "data directory must be an absolute path")
	}
	if req.ConfigPath == "" || !filepath.IsAbs(req.ConfigPath) {
		errs = append(errs, "config path must be an absolute path")
	} else if req.DataDir != "" && filepath.IsAbs(req.DataDir) {
		rel, err := filepath.Rel(req.DataDir, req.ConfigPath)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			errs = append(errs, "config path must be inside the data directory")
		}
	}
	if !validPort(req.ControllerPort) {
		errs = append(errs, "controller port must be between 1 and 65535")
	}
	if !validPort(req.MixedPort) {
		errs = append(errs, "mixed port must be between 1 and 65535")
	}
	if req.ControllerPort == req.MixedPort && req.ControllerPort != 0 {
		errs = append(errs, "controller and mixed ports must differ")
	}
	if req.Secret == "" || strings.ContainsAny(req.Secret, "\r\n") {
		errs = append(errs, "controller secret is required and must be a single line")
	}
	if len(errs) != 0 {
		return Request{}, errors.New("invalid core request: " + strings.Join(errs, "; "))
	}

	var err error
	req.MihomoPath, err = filepath.Abs(req.MihomoPath)
	if err != nil {
		return Request{}, fmt.Errorf("resolve mihomo path: %w", err)
	}
	req.DataDir, err = filepath.Abs(req.DataDir)
	if err != nil {
		return Request{}, fmt.Errorf("resolve data directory: %w", err)
	}
	req.ConfigPath, err = filepath.Abs(req.ConfigPath)
	if err != nil {
		return Request{}, fmt.Errorf("resolve config path: %w", err)
	}
	for _, path := range []string{req.MihomoPath, req.ConfigPath} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return Request{}, fmt.Errorf("stat request file: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			return Request{}, fmt.Errorf("request file is not regular: %s", path)
		}
	}
	info, err := os.Stat(req.DataDir)
	if err != nil {
		return Request{}, fmt.Errorf("stat data directory: %w", err)
	}
	if !info.IsDir() {
		return Request{}, fmt.Errorf("data path is not a directory: %s", req.DataDir)
	}
	return req, nil
}

func validPort(port int) bool {
	return port >= 1 && port <= 65535
}

func readAndRemoveRequest(path string) (Request, error) {
	if strings.TrimSpace(path) == "" {
		return Request{}, errors.New("core request path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Request{}, fmt.Errorf("read core request: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Request{}, fmt.Errorf("remove core request: %w", err)
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return Request{}, fmt.Errorf("decode core request: %w", err)
	}
	return validateRequest(req)
}
