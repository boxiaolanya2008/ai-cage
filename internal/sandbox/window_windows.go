//go:build windows

package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

const (
	className         = "AI Cage Sandbox Window"
	windowWaitTimeout = 10 * time.Second
)

var (
	classOnce sync.Once
	classErr  error
	wndProcCB = syscall.NewCallback(sandboxWndProc)
	setParent = windows.NewLazySystemDLL("user32.dll").NewProc("SetParent")
)

type sandboxState struct {
	host  w32.HWND
	child w32.HWND
	cmd   *exec.Cmd
}

// Open launches app and hosts its first visible top-level window inside a
// dedicated native AI Cage window. It is a window host, not a security sandbox.
func Open(app model.LaunchApp) error {
	executable := strings.TrimSpace(app.Path)
	if executable == "" {
		return errors.New("sandbox: executable path is empty")
	}
	if !strings.EqualFold(filepath.Ext(executable), ".exe") {
		return errors.New("sandbox: executable must be an .exe file")
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("sandbox: resolve executable: %w", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		return fmt.Errorf("sandbox: executable does not exist: %w", err)
	}

	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		run(absolute, app, ready)
	}()
	return <-ready
}

func run(executable string, app model.LaunchApp, ready chan<- error) {
	state := &sandboxState{}
	if err := registerClass(); err != nil {
		ready <- err
		return
	}

	title, err := windows.UTF16PtrFromString("AI Cage - " + app.Name)
	if err != nil {
		ready <- err
		return
	}
	class, err := windows.UTF16PtrFromString(className)
	if err != nil {
		ready <- err
		return
	}
	host := w32.CreateWindowEx(
		0,
		class,
		title,
		w32.WS_OVERLAPPEDWINDOW,
		w32.CW_USEDEFAULT,
		w32.CW_USEDEFAULT,
		1200,
		800,
		0,
		0,
		w32.GetModuleHandle(""),
		unsafe.Pointer(state),
	)
	if host == 0 {
		ready <- fmt.Errorf("sandbox: create host window failed: %w", windows.GetLastError())
		return
	}
	state.host = host
	w32.ShowWindow(host, w32.SW_SHOW)
	w32.UpdateWindow(host)

	cmd := exec.Command(executable, app.Args...)
	cmd.Dir = filepath.Dir(executable)
	before := snapshotTopLevelWindows(host)
	if err := cmd.Start(); err != nil {
		w32.DestroyWindow(host)
		ready <- fmt.Errorf("sandbox: start application: %w", err)
		return
	}
	state.cmd = cmd
	child, err := waitForNewWindow(before, host, filepath.Base(executable), cmd.Process.Pid, windowWaitTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		w32.DestroyWindow(host)
		ready <- err
		return
	}
	state.child = child
	embedChild(host, child)
	go monitorChildWindow(host, child)
	ready <- nil

	var msg w32.MSG
	for {
		ret := w32.GetMessage(&msg, 0, 0, 0)
		if ret <= 0 {
			break
		}
		w32.TranslateMessage(&msg)
		w32.DispatchMessage(&msg)
	}
}

func registerClass() error {
	classOnce.Do(func() {
		name, err := windows.UTF16PtrFromString(className)
		if err != nil {
			classErr = err
			return
		}
		instance := w32.GetModuleHandle("")
		wc := w32.WNDCLASSEX{
			Size:       uint32(unsafe.Sizeof(w32.WNDCLASSEX{})),
			Style:      w32.CS_HREDRAW | w32.CS_VREDRAW,
			WndProc:    wndProcCB,
			Instance:   instance,
			Icon:       w32.LoadIconWithResourceID(0, w32.IDI_APPLICATION),
			Cursor:     w32.LoadCursorWithResourceID(0, w32.IDC_ARROW),
			Background: w32.GetSysColorBrush(w32.COLOR_WINDOW),
			ClassName:  name,
			IconSm:     w32.LoadIconWithResourceID(0, w32.IDI_APPLICATION),
		}
		if w32.RegisterClassEx(&wc) == 0 {
			classErr = fmt.Errorf("sandbox: register window class failed: %w", windows.GetLastError())
		}
	})
	return classErr
}

func sandboxWndProc(hwnd w32.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == w32.WM_NCCREATE {
		create := (*w32.CREATESTRUCT)(uintptrToPointer(lParam))
		state := (*sandboxState)(uintptrToPointer(create.CreateParams))
		w32.SetWindowLongPtr(hwnd, w32.GWLP_USERDATA, uintptr(unsafe.Pointer(state)))
		return w32.DefWindowProc(hwnd, msg, wParam, lParam)
	}

	statePtr := w32.GetWindowLongPtr(hwnd, w32.GWLP_USERDATA)
	if statePtr != 0 {
		state := (*sandboxState)(uintptrToPointer(statePtr))
		switch msg {
		case w32.WM_SIZE:
			resizeChild(state)
		case w32.WM_CLOSE:
			if state.child != 0 && w32.IsWindow(state.child) {
				w32.PostMessage(state.child, w32.WM_CLOSE, 0, 0)
			}
			w32.DestroyWindow(hwnd)
			return 0
		case w32.WM_DESTROY:
			w32.SetWindowLongPtr(hwnd, w32.GWLP_USERDATA, 0)
			w32.PostQuitMessage(0)
			return 0
		}
	}
	return w32.DefWindowProc(hwnd, msg, wParam, lParam)
}

func uintptrToPointer(v uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&v))
}

func snapshotTopLevelWindows(exclude w32.HWND) map[w32.HWND]struct{} {
	windowsSet := make(map[w32.HWND]struct{})
	callback := syscall.NewCallback(func(hwnd w32.HWND, _ uintptr) uintptr {
		if hwnd != exclude {
			windowsSet[hwnd] = struct{}{}
		}
		return 1
	})
	w32.EnumWindows(callback, 0)
	return windowsSet
}

func waitForNewWindow(before map[w32.HWND]struct{}, host w32.HWND, imageBase string, pid int, timeout time.Duration) (w32.HWND, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var found w32.HWND
		callback := syscall.NewCallback(func(hwnd w32.HWND, _ uintptr) uintptr {
			if hwnd == host {
				return 1
			}
			if _, exists := before[hwnd]; exists {
				return 1
			}
			if !w32.IsWindowVisible(hwnd) {
				return 1
			}
			_, windowPID := w32.GetWindowThreadProcessId(hwnd)
			if windowPID == pid {
				found = hwnd
				return 0
			}
			if strings.EqualFold(processImageBase(windowPID), imageBase) {
				found = hwnd
				return 0
			}
			return 1
		})
		w32.EnumWindows(callback, 0)
		if found != 0 {
			return found, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("sandbox: application did not create a visible window within %s", timeout)
}

func processImageBase(pid int) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buffer[:size]))
}

func monitorChildWindow(host, child w32.HWND) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if !w32.IsWindow(child) {
			if w32.IsWindow(host) {
				w32.PostMessage(host, w32.WM_CLOSE, 0, 0)
			}
			return
		}
	}
}

func embedChild(host, child w32.HWND) {
	style := w32.GetWindowLongPtr(child, w32.GWL_STYLE)
	style &^= w32.WS_POPUP
	style |= w32.WS_CHILD
	w32.SetWindowLongPtr(child, w32.GWL_STYLE, style)
	setParent.Call(uintptr(child), uintptr(host))
	w32.ShowWindow(child, w32.SW_SHOW)
	resizeChildToHost(host, child)
}

func resizeChild(state *sandboxState) {
	if state == nil || state.host == 0 || state.child == 0 {
		return
	}
	resizeChildToHost(state.host, state.child)
}

func resizeChildToHost(host, child w32.HWND) {
	rect := w32.GetClientRect(host)
	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)
	w32.SetWindowPos(child, 0, 0, 0, width, height, w32.SWP_NOZORDER|w32.SWP_NOACTIVATE|w32.SWP_FRAMECHANGED)
}
