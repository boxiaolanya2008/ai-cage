//go:build !windows

package core

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func launchPlatform(executable, requestPath string, elevated bool) error {
	return launchOther(executable, requestPath, elevated)
}

func isElevated() bool { return false }

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
