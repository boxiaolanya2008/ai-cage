package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Launch writes a private request file and starts this executable in core mode.
// On Windows elevated launches use ShellExecuteEx with the runas verb.
func Launch(req Request, elevated bool) error {
	req, err := validateRequest(req)
	if err != nil {
		return err
	}
	path, err := connectorPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create core request directory: %w", err)
	}
	// Do not let the UI mistake a previous connector for this new process.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale core connector: %w", err)
	}
	file, err := os.CreateTemp(dir, "request-*.json")
	if err != nil {
		return fmt.Errorf("create core request: %w", err)
	}
	requestPath := file.Name()
	launched := false
	defer func() {
		_ = file.Close()
		if !launched {
			_ = os.Remove(requestPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure core request: %w", err)
	}
	if err := json.NewEncoder(file).Encode(req); err != nil {
		return fmt.Errorf("encode core request: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync core request: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close core request: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if err := launchPlatform(executable, requestPath, elevated); err != nil {
		return fmt.Errorf("launch core: %w", err)
	}
	launched = true
	return nil
}

func launchOther(executable, requestPath string, elevated bool) error {
	if elevated {
		return errors.New("elevated core launch is only supported on Windows")
	}
	cmd := exec.Command(executable, "--core", requestPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func requestCommandLine(requestPath string) string {
	// ShellExecuteEx accepts a raw command line rather than an argument slice.
	// Quote the single path argument using Windows command-line rules.
	return "--core " + strings.TrimSpace(quoteWindowsArg(requestPath))
}

func quoteWindowsArg(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\"") {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, r := range value {
		switch r {
		case '\\':
			slashes++
		case '"':
			b.WriteString(strings.Repeat("\\", slashes*2+1))
			b.WriteByte('"')
			slashes = 0
		default:
			b.WriteString(strings.Repeat("\\", slashes))
			slashes = 0
			b.WriteRune(r)
		}
	}
	b.WriteString(strings.Repeat("\\", slashes*2))
	b.WriteByte('"')
	return b.String()
}
