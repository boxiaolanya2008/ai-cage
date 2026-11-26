package launcher

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

const (
	helperEnv    = "AI_CAGE_LAUNCHER_HELPER"
	helperOutput = "AI_CAGE_LAUNCHER_OUTPUT"
)

func TestLaunchValidatesExecutable(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "program.txt")
	if err := os.WriteFile(textPath, []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	exeDir := filepath.Join(dir, "directory.exe")
	if err := os.Mkdir(exeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "missing", path: filepath.Join(dir, "missing.exe")},
		{name: "wrong extension", path: textPath},
		{name: "directory", path: exeDir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := New().Launch(model.LaunchApp{Path: tc.path}); err == nil {
				t.Fatalf("Launch accepted path %q", tc.path)
			}
		})
	}
	upperPath := filepath.Join(dir, "PROGRAM.EXE")
	if err := os.WriteFile(upperPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := validateExecutable(upperPath)
	if err != nil {
		t.Fatalf("uppercase .EXE rejected: %v", err)
	}
	absolute, _ := filepath.Abs(upperPath)
	if got != absolute {
		t.Fatalf("validateExecutable() = %q, want %q", got, absolute)
	}
}

func TestLaunchPassesArgumentsWithoutShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("launcher targets Windows executables")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(t.TempDir(), "launcher-helper.exe")
	copyExecutable(t, executable, helperPath)
	outputPath := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(helperEnv, "1")
	t.Setenv(helperOutput, outputPath)

	want := []string{"plain", "two words", `a&b`, `"quoted"`, `%PATH%`, `$(echo nope)`}
	args := append([]string{"-test.run=^TestLauncherHelperProcess$", "--"}, want...)
	app := model.LaunchApp{ID: "helper", Name: "Helper", Path: helperPath, Args: args}
	if err := New().Launch(app); err != nil {
		t.Fatal(err)
	}

	var data []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err = os.ReadFile(outputPath)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("helper did not write arguments: %v", err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode helper output %q: %v", data, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestLauncherHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	output := os.Getenv(helperOutput)
	if output == "" {
		os.Exit(2)
	}
	data, err := json.Marshal(flag.Args())
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(output, data, 0o600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func copyExecutable(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
