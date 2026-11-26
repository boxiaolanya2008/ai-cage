package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/boxiaolanya2008/ai-cage/internal/app"
	"github.com/boxiaolanya2008/ai-cage/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "--core" {
		if err := core.Run(os.Args[2]); err != nil {
			log.Printf("core stopped: %v", err)
			os.Exit(1)
		}
		return
	}
	if err := runUI(); err != nil {
		log.Printf("ui stopped: %v", err)
		os.Exit(1)
	}
}

func runUI() error {
	service, err := app.New()
	if err != nil {
		return err
	}
	defer service.Shutdown()

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "ai-cage",
		Description: "Windows proxy and TUN workspace manager",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "io.github.boxiaolanya2008.aicage.ui",
			EncryptionKey: [32]byte{0x41, 0x49, 0x43, 0x41, 0x47, 0x45, 0x2d, 0x53, 0x49, 0x4e, 0x47, 0x4c, 0x45, 0x2d, 0x55, 0x49, 0x2d, 0x4b, 0x45, 0x59, 0x2d, 0x32, 0x30, 0x32, 0x36, 0x21, 0x21, 0x21, 0x21, 0x21, 0x21, 0x21},
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window != nil {
					window.Restore()
					window.Focus()
				}
			},
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "main",
		Title:                      "AI Cage",
		Width:                      1180,
		Height:                     760,
		MinWidth:                   940,
		MinHeight:                  640,
		URL:                        "/",
		BackgroundColour:           application.NewRGB(10, 13, 20),
		DefaultContextMenuDisabled: true,
		EnableFileDrop:             false,
		Zoom:                       1,
		ZoomControlEnabled:         false,
		Windows: application.WindowsWindow{
			DisableIcon: false,
			Theme:       application.SystemDefault,
		},
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	tray := app.SystemTray.New().SetIcon(appIcon)
	menu := app.NewMenu()
	menu.Add("打开 AI Cage").OnClick(func(_ *application.Context) {
		window.Restore()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("启动代理").OnClick(func(_ *application.Context) {
		go func() {
			if _, err := service.StartProxy(false); err != nil {
				app.Dialog.Error().SetTitle("启动失败").SetMessage(err.Error()).Show()
			}
		}()
	})
	menu.Add("停止代理").OnClick(func(_ *application.Context) {
		go func() {
			if err := service.StopProxy(); err != nil {
				app.Dialog.Error().SetTitle("停止失败").SetMessage(err.Error()).Show()
			}
		}()
	})
	menu.AddSeparator()
	menu.Add("退出并停止 sidecar").OnClick(func(_ *application.Context) {
		if err := service.Shutdown(); err != nil {
			app.Dialog.Error().SetTitle("退出失败").SetMessage(err.Error()).Show()
			return
		}
		app.Quit()
	})
	tray.SetMenu(menu)

	if runtime.GOOS == "windows" {
		tray.SetTooltip("AI Cage")
	}
	return app.Run()
}
