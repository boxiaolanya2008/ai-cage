package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

// Launcher starts configured Windows applications without elevation.
type Launcher struct{}

// New creates a launcher.
func New() *Launcher {
	return &Launcher{}
}

// Launch validates app.Path and starts it with app.Args as separate arguments.
// It does not invoke a shell or request administrator privileges.
func (l *Launcher) Launch(app model.LaunchApp) error {
	executable, err := validateExecutable(app.Path)
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, app.Args...)
	cmd.Path = executable
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launcher: start %q: %w", executable, err)
	}
	// The caller does not receive the process handle, so release it immediately.
	_ = cmd.Process.Release()
	return nil
}

func validateExecutable(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("launcher: executable path is empty")
	}
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", errors.New("launcher: executable must have an .exe extension")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("launcher: resolve executable path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("launcher: executable does not exist: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("launcher: executable path is not a regular file")
	}
	return absolute, nil
}
