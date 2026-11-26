package verge

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	appDirName    = "io.github.clash-verge-rev.clash-verge-rev"
	runtimeFile   = "clash-verge.yaml"
	profilesFile  = "profiles.yaml"
	profilesDir   = "profiles"
	maxConfigSize = 16 << 20
)

var managedKeys = []string{
	"port",
	"socks-port",
	"redir-port",
	"tproxy-port",
	"mixed-port",
	"allow-lan",
	"mode",
	"log-level",
	"external-controller",
	"external-controller-cors",
	"external-controller-pipe",
	"external-controller-tls",
	"external-controller-unix",
	"external-ui",
	"secret",
	"tun",
	"dns",
	"ipv6",
	"find-process-mode",
}

// AppDir returns the fixed Clash Verge Rev data directory.
func AppDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// Snapshot reads the current Clash Verge configuration without writing to
// Verge. It prefers the generated runtime file and falls back to the profile
// selected in profiles.yaml.
func Snapshot() ([]byte, error) {
	dir, err := AppDir()
	if err != nil {
		return nil, err
	}
	return snapshotFromDir(dir)
}

func snapshotFromDir(dir string) ([]byte, error) {
	runtimePath := filepath.Join(dir, runtimeFile)
	if info, err := os.Stat(runtimePath); err == nil && info.Mode().IsRegular() {
		data, readErr := readBounded(runtimePath)
		if readErr != nil {
			return nil, fmt.Errorf("read Clash Verge runtime config: %w", readErr)
		}
		return Sanitize(data)
	}

	profilePath, err := currentProfilePath(dir)
	if err != nil {
		return nil, err
	}
	data, err := readBounded(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read Clash Verge profile: %w", err)
	}
	return Sanitize(data)
}

// Sanitize removes fields owned by Clash Verge or AI Cage while retaining the
// subscription's proxies, groups, rules, and providers.
func Sanitize(raw []byte) ([]byte, error) {
	var config map[string]any
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse Clash Verge config: %w", err)
	}
	if config == nil {
		return nil, errors.New("Clash Verge config is empty")
	}
	for _, key := range managedKeys {
		delete(config, key)
	}
	removePlaceholderProxies(config)
	if !hasProxies(config) {
		return nil, errors.New("Clash Verge config has no usable proxies or proxy-providers")
	}
	merged, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode Clash Verge snapshot: %w", err)
	}
	return merged, nil
}

func currentProfilePath(dir string) (string, error) {
	data, err := readBounded(filepath.Join(dir, profilesFile))
	if err != nil {
		return "", fmt.Errorf("read Clash Verge profiles index: %w", err)
	}
	var profiles struct {
		Current string `yaml:"current"`
	}
	if err := yaml.Unmarshal(data, &profiles); err != nil {
		return "", fmt.Errorf("parse Clash Verge profiles index: %w", err)
	}
	current := strings.TrimSpace(profiles.Current)
	if current == "" || strings.ContainsAny(current, `\/`) {
		return "", errors.New("Clash Verge current profile is invalid")
	}
	path := filepath.Join(dir, profilesDir, current+".yaml")
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Clash Verge current profile does not exist: %s", current)
	}
	return path, nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxConfigSize {
		return nil, fmt.Errorf("config exceeds %d bytes", maxConfigSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("config exceeds %d bytes", maxConfigSize)
	}
	return data, nil
}

func removePlaceholderProxies(config map[string]any) {
	proxies := yamlArray(config, "proxies")
	usable := make([]any, 0, len(proxies))
	removed := make(map[string]struct{})
	for _, item := range proxies {
		proxy, ok := item.(map[string]any)
		if !ok {
			usable = append(usable, item)
			continue
		}
		name, _ := proxy["name"].(string)
		server, _ := proxy["server"].(string)
		if server == "10.255.255.255" {
			removed[name] = struct{}{}
			continue
		}
		usable = append(usable, proxy)
	}
	config["proxies"] = usable

	groups := yamlArray(config, "proxy-groups")
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		members := yamlArray(group, "proxies")
		cleaned := make([]any, 0, len(members))
		for _, member := range members {
			name, _ := member.(string)
			if _, exists := removed[name]; exists {
				continue
			}
			cleaned = append(cleaned, member)
		}
		group["proxies"] = cleaned
	}
}

func yamlArray(parent map[string]any, key string) []any {
	value, ok := parent[key]
	if !ok || value == nil {
		return []any{}
	}
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result
	default:
		return []any{}
	}
}

func hasProxies(config map[string]any) bool {
	for _, key := range []string{"proxies", "proxy-providers"} {
		switch typed := config[key].(type) {
		case []any:
			if len(typed) > 0 {
				return true
			}
		case map[string]any:
			if len(typed) > 0 {
				return true
			}
		}
	}
	return false
}
