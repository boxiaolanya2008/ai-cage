//go:build windows

package mihomo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}

func createProcessJob() (*processJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create mihomo Job Object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("configure mihomo Job Object: %w", err)
	}
	return &processJob{
		close: func() error { return windows.CloseHandle(handle) },
		assign: func(process *os.Process) error {
			var assignErr error
			if err := process.WithHandle(func(processHandle uintptr) {
				assignErr = windows.AssignProcessToJobObject(handle, windows.Handle(processHandle))
			}); err != nil {
				return err
			}
			return assignErr
		},
	}, nil
}

func assignProcessToJob(job *processJob, process *os.Process) error {
	if job == nil || job.assign == nil {
		return nil
	}
	return job.assign(process)
}

func closeProcessJob(job *processJob) error {
	if job == nil || job.close == nil {
		return nil
	}
	job.once.Do(func() { job.closeErr = job.close() })
	return job.closeErr
}

func gracefulStop(ctx context.Context, process *os.Process) error {
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(process.Pid)); err == nil {
		return nil
	}
	// taskkill without /F requests graceful termination and includes children.
	// Resolve the system binary instead of trusting an inherited PATH.
	taskkill := "taskkill.exe"
	if root := os.Getenv("SystemRoot"); root != "" {
		taskkill = filepath.Join(root, "System32", "taskkill.exe")
	}
	return exec.CommandContext(ctx, taskkill, "/PID", fmt.Sprint(process.Pid), "/T").Run()
}

func forceStop(job *processJob, process *os.Process) error {
	var errs []error
	if job != nil {
		if err := closeProcessJob(job); err != nil && !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
			errs = append(errs, err)
		}
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
