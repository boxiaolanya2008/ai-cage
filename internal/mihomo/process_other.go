//go:build !windows

package mihomo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func createProcessJob() (*processJob, error) { return nil, nil }

func assignProcessToJob(_ *processJob, _ *os.Process) error { return nil }

func closeProcessJob(_ *processJob) error { return nil }

func gracefulStop(ctx context.Context, process *os.Process) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return syscall.Kill(-process.Pid, syscall.SIGTERM)
}

func forceStop(_ *processJob, process *os.Process) error {
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
