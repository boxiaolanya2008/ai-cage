package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/config"
	"github.com/boxiaolanya2008/ai-cage/internal/core"
	"github.com/boxiaolanya2008/ai-cage/internal/launcher"
	"github.com/boxiaolanya2008/ai-cage/internal/mihomo"
	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"github.com/boxiaolanya2008/ai-cage/internal/sandbox"
	"github.com/boxiaolanya2008/ai-cage/internal/subscription"
	"github.com/boxiaolanya2008/ai-cage/internal/verge"
)

type AppService struct {
	store     *config.Store
	subs      *subscription.Manager
	launcher  *launcher.Launcher
	settings  model.Settings
	client    *mihomo.Client
	core      *core.Client
	connector core.Connector

	opMu sync.Mutex
	mu   sync.RWMutex
}

func New() (*AppService, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate user config directory: %w", err)
	}
	store, err := config.NewStore(filepath.Join(base, "ai-cage"))
	if err != nil {
		return nil, err
	}
	subs, err := subscription.NewManager(store)
	if err != nil {
		return nil, err
	}
	settings, err := store.LoadSettings()
	if err != nil {
		return nil, err
	}
	return &AppService{
		store:    store,
		subs:     subs,
		launcher: launcher.New(),
		settings: settings,
	}, nil
}

func (a *AppService) GetSettings() (model.Settings, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings, nil
}

func (a *AppService) SaveSettings(settings model.Settings) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	if a.client != nil {
		return errors.New("请先停止当前代理连接，再修改运行设置")
	}
	if settings.KernelMode != model.KernelModeVerge && settings.KernelMode != model.KernelModeSidecar {
		return errors.New("未知内核模式")
	}
	if err := validatePort("sidecar controller", settings.Sidecar.ControllerPort); err != nil {
		return err
	}
	if err := validatePort("sidecar mixed", settings.Sidecar.MixedPort); err != nil {
		return err
	}
	if settings.Sidecar.ControllerPort == settings.Sidecar.MixedPort {
		return errors.New("sidecar Controller 端口与混合代理端口不能相同")
	}
	if _, err := mihomo.NewClient(settings.Verge.ControllerURL, settings.Verge.Secret); err != nil {
		return fmt.Errorf("Verge Controller 地址无效: %w", err)
	}
	if settings.KernelMode == model.KernelModeVerge {
		secret := strings.TrimSpace(settings.Verge.Secret)
		if secret == "" || secret == "set-your-secret" {
			return errors.New("Verge Secret 不能为空，也不能使用公开占位值 set-your-secret")
		}
	}
	if settings.TUN.DeviceName == "" {
		return errors.New("TUN 设备名不能为空")
	}
	if !validDNS(settings.TUN.PrimaryDNS) || !validDNS(settings.TUN.SecondaryDNS) {
		return errors.New("TUN DNS 必须是有效的 IPv4 地址")
	}
	if settings.KernelMode == model.KernelModeVerge {
		settings.TUN.Enabled = false
	}
	if err := a.store.SaveSettings(settings); err != nil {
		return err
	}
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
	return nil
}

func (a *AppService) TestConnection() (model.Snapshot, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	settings := a.currentSettings()
	if a.client != nil {
		return a.snapshotWithClient(settings, a.client, a.coreElevated())
	}
	client, err := a.clientForSettings(settings)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	return a.snapshotWithClient(settings, client, false)
}

func (a *AppService) StartProxy(confirmTUN bool) (model.Snapshot, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	settings := a.currentSettings()
	if a.client != nil {
		return a.snapshotWithClient(settings, a.client, a.coreElevated())
	}

	if settings.KernelMode == model.KernelModeVerge {
		if err := validateVergeSecret(settings.Verge.Secret); err != nil {
			return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
		}
		if settings.TUN.Enabled {
			return model.Snapshot{Mode: settings.KernelMode, Message: "Verge 模式的 TUN 由 Clash Verge 管理"}, errors.New("Verge 模式下请关闭 ai-cage 的 TUN 开关；TUN 请在 Clash Verge 中操作")
		}
		client, err := a.clientForSettings(settings)
		if err != nil {
			return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
		}
		snapshot, err := a.snapshotWithClient(settings, client, false)
		if err != nil {
			return snapshot, err
		}
		a.mu.Lock()
		a.client = client
		a.mu.Unlock()
		return snapshot, nil
	}

	if settings.TUN.Enabled && !confirmTUN {
		return model.Snapshot{Mode: settings.KernelMode, Message: "需要确认 Verge TUN 已关闭"}, errors.New("启用 sidecar TUN 前，请确认 Clash Verge 的 TUN 已关闭")
	}
	if settings.Sidecar.ActiveSubscription == "" {
		return model.Snapshot{Mode: settings.KernelMode, Message: "尚未选择订阅"}, errors.New("请先添加并启用一个订阅")
	}
	raw, err := a.subs.ReadConfig(settings.Sidecar.ActiveSubscription)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, fmt.Errorf("read active subscription: %w", err)
	}
	apps, err := a.store.ListApps()
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, fmt.Errorf("list caged apps: %w", err)
	}
	secret, err := newSecret()
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	runtimeConfig, err := mihomo.MergeConfig(raw, settings.Sidecar.ControllerPort, settings.Sidecar.MixedPort, secret, settings.TUN, apps, settings.Sidecar.LogLevel)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, fmt.Errorf("merge runtime config: %w", err)
	}
	configPath := filepath.Join(a.store.DataDir(), "runtime.yaml")
	if err := writeFileAtomic(configPath, runtimeConfig, 0o600); err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}

	request := core.Request{
		MihomoPath:     settings.Sidecar.MihomoPath,
		MihomoSHA256:   settings.Sidecar.MihomoSHA256,
		DataDir:        a.store.DataDir(),
		ConfigPath:     configPath,
		ControllerPort: settings.Sidecar.ControllerPort,
		MixedPort:      settings.Sidecar.MixedPort,
		Secret:         secret,
	}
	if err := core.Launch(request, settings.TUN.Enabled); err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}

	connector, err := waitForCore(15 * time.Second)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	coreClient := core.NewClient(connector)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := coreClient.Start(ctx); err != nil {
		_ = coreClient.Shutdown(context.Background())
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	controllerURL := fmt.Sprintf("http://127.0.0.1:%d", settings.Sidecar.ControllerPort)
	client, err := mihomo.NewClient(controllerURL, secret)
	if err != nil {
		_ = coreClient.Stop(context.Background())
		_ = coreClient.Shutdown(context.Background())
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	snapshot, err := a.snapshotWithClient(settings, client, connector.Elevated)
	if err != nil {
		_ = coreClient.Stop(context.Background())
		_ = coreClient.Shutdown(context.Background())
		return snapshot, err
	}
	a.mu.Lock()
	a.client = client
	a.core = coreClient
	a.connector = connector
	a.mu.Unlock()
	return snapshot, nil
}

func (a *AppService) StopProxy() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.RLock()
	client, coreClient := a.client, a.core
	mode := a.settings.KernelMode
	a.mu.RUnlock()
	if client == nil {
		return nil
	}
	if mode == model.KernelModeVerge {
		a.clearConnection()
		return nil
	}
	if coreClient == nil {
		return errors.New("sidecar core 连接不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := coreClient.Stop(ctx); err != nil {
		return err
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := coreClient.Shutdown(shutdownCtx); err != nil {
		return err
	}
	a.clearConnection()
	return nil
}

func (a *AppService) GetSnapshot() (model.Snapshot, error) {
	a.mu.RLock()
	client := a.client
	settings := a.settings
	elevated := a.connector.Elevated
	a.mu.RUnlock()
	if client == nil {
		return model.Snapshot{Mode: settings.KernelMode, Running: false, Message: "未连接"}, nil
	}
	return a.snapshotWithClient(settings, client, elevated)
}

func (a *AppService) SetProxyMode(mode model.ProxyMode) error {
	client, err := a.requireClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return client.SetProxyMode(ctx, mode)
}

func (a *AppService) GetProxies() (model.Proxies, error) {
	client, err := a.requireClient()
	if err != nil {
		return model.Proxies{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return client.Proxies(ctx)
}

func (a *AppService) SelectProxy(group, name string) error {
	client, err := a.requireClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return client.SelectProxy(ctx, group, name)
}

func (a *AppService) TestProxyDelay(group, name string) (model.DelayResult, error) {
	client, err := a.requireClient()
	if err != nil {
		return model.DelayResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.Delay(ctx, "http://www.gstatic.com/generate_204", name, 5000)
}

func (a *AppService) ListSubscriptions() ([]model.Subscription, error) {
	return a.subs.List()
}

func (a *AppService) ImportVergeSnapshot() (model.Subscription, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	if a.client != nil {
		return model.Subscription{}, errors.New("请先停止当前代理连接，再导入 Verge 配置")
	}
	snapshot, err := verge.Snapshot()
	if err != nil {
		return model.Subscription{}, err
	}
	item, err := a.subs.ImportSnapshot("Clash Verge 当前配置", snapshot)
	if err != nil {
		return model.Subscription{}, err
	}
	settings := a.currentSettings()
	settings.KernelMode = model.KernelModeSidecar
	settings.Sidecar.ActiveSubscription = item.ID
	settings.TUN.Enabled = true
	settings.TUN.OnlyCagedApps = true
	if err := a.store.SaveSettings(settings); err != nil {
		return model.Subscription{}, err
	}
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
	return item, nil
}

func (a *AppService) CheckExitIP() (string, error) {
	client, err := a.requireClient()
	if err != nil {
		return "", err
	}
	settings := a.currentSettings()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.ExitIP(ctx, settings.Sidecar.MixedPort)
}

func (a *AppService) AddSubscription(name, url string) (model.Subscription, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	item, err := a.subs.Add(name, url)
	if err != nil {
		return model.Subscription{}, err
	}
	settings := a.currentSettings()
	if settings.Sidecar.ActiveSubscription == "" {
		settings.Sidecar.ActiveSubscription = item.ID
		if err := a.store.SaveSettings(settings); err != nil {
			return model.Subscription{}, err
		}
		a.mu.Lock()
		a.settings = settings
		a.mu.Unlock()
	}
	return item, nil
}

func (a *AppService) UpdateSubscription(id string) (model.Subscription, error) {
	return a.subs.Update(id)
}

func (a *AppService) DeleteSubscription(id string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	if err := a.subs.Delete(id); err != nil {
		return err
	}
	settings := a.currentSettings()
	if settings.Sidecar.ActiveSubscription == id {
		settings.Sidecar.ActiveSubscription = ""
		if err := a.store.SaveSettings(settings); err != nil {
			return err
		}
		a.mu.Lock()
		a.settings = settings
		a.mu.Unlock()
	}
	return nil
}

func (a *AppService) UseSubscription(id string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	items, err := a.subs.List()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == id {
			settings := a.currentSettings()
			settings.Sidecar.ActiveSubscription = id
			if err := a.store.SaveSettings(settings); err != nil {
				return err
			}
			a.mu.Lock()
			a.settings = settings
			a.mu.Unlock()
			return nil
		}
	}
	return errors.New("订阅不存在")
}

func (a *AppService) ListApps() ([]model.LaunchApp, error) {
	return a.store.ListApps()
}

func (a *AppService) SaveApp(item model.LaunchApp) error {
	return a.store.SaveApp(item)
}

func (a *AppService) DeleteApp(id string) error {
	return a.store.DeleteApp(id)
}

func (a *AppService) LaunchApp(id string) error {
	items, err := a.store.ListApps()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == id {
			return a.launcher.Launch(item)
		}
	}
	return errors.New("应用不存在")
}

func (a *AppService) OpenSandboxWindow(id string) error {
	items, err := a.store.ListApps()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == id {
			return sandbox.Open(item)
		}
	}
	return errors.New("应用不存在")
}

func (a *AppService) CalculateMihomoSHA256(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("mihomo 路径不能为空")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open mihomo: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash mihomo: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (a *AppService) Shutdown() error {
	return a.StopProxy()
}

func (a *AppService) currentSettings() model.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

func (a *AppService) clientForSettings(settings model.Settings) (*mihomo.Client, error) {
	if settings.KernelMode == model.KernelModeVerge {
		return mihomo.NewClient(settings.Verge.ControllerURL, settings.Verge.Secret)
	}
	return nil, errors.New("独立内核尚未启动")
}

func (a *AppService) snapshotWithClient(settings model.Settings, client *mihomo.Client, elevated bool) (model.Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	version, err := client.Version(ctx)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Message: err.Error()}, err
	}
	runningConfig, err := client.Config(ctx)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Version: version, Message: err.Error()}, err
	}
	connections, err := client.Connections(ctx)
	if err != nil {
		return model.Snapshot{Mode: settings.KernelMode, Version: version, ProxyMode: model.ProxyMode(runningConfig.Mode), TUNEnabled: runningConfig.TUN.Enabled, Message: err.Error()}, err
	}
	traffic := model.Traffic{Down: connections.DownloadTotal, Up: connections.UploadTotal, DownTotal: connections.DownloadTotal, UpTotal: connections.UploadTotal}
	if sample, ok := readTraffic(client); ok {
		traffic = sample
	}
	controllerURL := settings.Verge.ControllerURL
	if settings.KernelMode == model.KernelModeSidecar {
		controllerURL = fmt.Sprintf("http://127.0.0.1:%d", settings.Sidecar.ControllerPort)
	}
	return model.Snapshot{
		Mode:            settings.KernelMode,
		Running:         true,
		CoreElevated:    elevated,
		ControllerURL:   controllerURL,
		Version:         version,
		ProxyMode:       model.ProxyMode(runningConfig.Mode),
		TUNEnabled:      runningConfig.TUN.Enabled,
		Traffic:         traffic,
		ConnectionCount: len(connections.Connections),
		Message:         "已连接",
	}, nil
}

func readTraffic(client *mihomo.Client) (model.Traffic, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	traffic, _, stop, err := client.Traffic(ctx)
	if err != nil {
		return model.Traffic{}, false
	}
	defer stop()
	select {
	case sample, ok := <-traffic:
		return sample, ok
	case <-ctx.Done():
		return model.Traffic{}, false
	}
}

func (a *AppService) requireClient() (*mihomo.Client, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.client == nil {
		return nil, errors.New("请先启动代理连接")
	}
	return a.client, nil
}

func (a *AppService) coreElevated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connector.Elevated
}

func (a *AppService) clearConnection() {
	a.mu.Lock()
	a.client = nil
	a.core = nil
	a.connector = core.Connector{}
	a.mu.Unlock()
}

func waitForCore(timeout time.Duration) (core.Connector, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		connector, err := core.ReadConnector()
		if err == nil {
			return connector, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return core.Connector{}, fmt.Errorf("等待提权 core 启动超时: %v", lastErr)
}

func newSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate controller secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validateVergeSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" || secret == "set-your-secret" {
		return errors.New("Verge Secret 不能为空，也不能使用公开占位值 set-your-secret")
	}
	return nil
}

func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s 端口必须在 1-65535 之间", name)
	}
	return nil
}

func validDNS(value string) bool {
	ip := net.ParseIP(value)
	return ip != nil && ip.To4() != nil
}
