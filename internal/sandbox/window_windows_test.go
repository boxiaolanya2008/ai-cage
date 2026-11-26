//go:build windows

package sandbox

import (
	"strings"
	"testing"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

func TestOpenRejectsInvalidExecutable(t *testing.T) {
	tests := []struct {
		name string
		app  model.LaunchApp
		want string
	}{
		{name: "empty path", app: model.LaunchApp{Name: "Empty"}, want: "executable path is empty"},
		{name: "non exe", app: model.LaunchApp{Name: "Text", Path: "notepad.txt"}, want: "must be an .exe file"},
		{name: "missing exe", app: model.LaunchApp{Name: "Missing", Path: `C:\does-not-exist.exe`}, want: "executable does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Open(tt.app)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Open() error = %v, want contains %q", err, tt.want)
			}
		})
	}
}
