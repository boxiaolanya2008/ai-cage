package mihomo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"gopkg.in/yaml.v3"
)

const aiCageGroup = "AI-CAGE"

// MergeConfig applies the settings managed by ai-cage while retaining the
// subscription's proxies, groups, rules, providers, and other YAML fields.
func MergeConfig(raw []byte, controllerPort, mixedPort int, secret string, tun model.TUNSettings, apps []model.LaunchApp, logLevel string) ([]byte, error) {
	if !validPort(controllerPort) {
		return nil, fmt.Errorf("controller port must be between 1 and 65535: %d", controllerPort)
	}
	if !validPort(mixedPort) {
		return nil, fmt.Errorf("mixed port must be between 1 and 65535: %d", mixedPort)
	}
	if strings.TrimSpace(tun.PrimaryDNS) == "" || strings.TrimSpace(tun.SecondaryDNS) == "" {
		return nil, fmt.Errorf("primary and secondary DNS servers are required")
	}

	config := make(map[string]any)
	if len(strings.TrimSpace(string(raw))) != 0 {
		if err := yaml.Unmarshal(raw, &config); err != nil {
			return nil, fmt.Errorf("parse mihomo config: %w", err)
		}
	}
	if config == nil {
		config = make(map[string]any)
	}

	// Disable every subscription inbound except the sidecar's own mixed port.
	config["port"] = 0
	config["socks-port"] = 0
	config["redir-port"] = 0
	config["tproxy-port"] = 0
	config["mixed-port"] = mixedPort
	config["allow-lan"] = false
	config["mode"] = "rule"
	config["log-level"] = logLevel
	config["external-controller"] = fmt.Sprintf("127.0.0.1:%d", controllerPort)
	config["secret"] = secret
	config["ipv6"] = tun.IPv6

	tunConfig := yamlObject(config, "tun")
	tunConfig["enable"] = tun.Enabled
	if tun.Enabled {
		tunConfig["device"] = tun.DeviceName
		tunConfig["stack"] = tun.Stack
		tunConfig["auto-route"] = tun.AutoRoute
		tunConfig["strict-route"] = tun.StrictRoute
		tunConfig["auto-detect-interface"] = tun.AutoDetectInterface
		tunConfig["dns-hijack"] = []string{"any:53"}
	} else {
		// Do not leave DNS hijacking active when managed TUN is disabled.
		tunConfig["dns-hijack"] = []string{}
	}
	delete(config, "dns-hijack")

	dns := yamlObject(config, "dns")
	dns["enable"] = true
	dns["enhanced-mode"] = "fake-ip"
	dns["fake-ip-range"] = "198.18.0.1/16"
	dns["nameserver"] = []string{strings.TrimSpace(tun.PrimaryDNS), strings.TrimSpace(tun.SecondaryDNS)}

	prepareAICageGroup(config)
	if tun.OnlyCagedApps {
		config["find-process-mode"] = "always"
		prependProcessRules(config, processPaths(apps))
		replaceFinalMatch(config, "DIRECT")
	} else {
		config["find-process-mode"] = "strict"
	}

	merged, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal mihomo config: %w", err)
	}
	return merged, nil
}

func prepareAICageGroup(config map[string]any) {
	groups := yamlArray(config, "proxy-groups")
	cleaned := make([]any, 0, len(groups)+1)
	upstreams := make([]any, 0, len(groups))
	for _, group := range groups {
		object, ok := group.(map[string]any)
		if !ok {
			cleaned = append(cleaned, group)
			continue
		}
		name, _ := object["name"].(string)
		if name == aiCageGroup {
			continue
		}
		if name != "" {
			upstreams = append(upstreams, name)
		}
		cleaned = append(cleaned, group)
	}
	if len(upstreams) == 0 {
		upstreams = []any{"DIRECT"}
	}
	aiCage := map[string]any{
		"name":    aiCageGroup,
		"type":    "select",
		"proxies": upstreams,
	}
	config["proxy-groups"] = append([]any{aiCage}, cleaned...)
}

func prependProcessRules(config map[string]any, paths []string) {
	if len(paths) == 0 {
		return
	}
	rules := yamlArray(config, "rules")
	processRules := make([]any, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		processRules = append(processRules, "PROCESS-PATH,"+path+","+aiCageGroup)
	}
	if len(processRules) == 0 {
		return
	}
	config["rules"] = append(processRules, rules...)
}

func replaceFinalMatch(config map[string]any, target string) {
	rules := yamlArray(config, "rules")
	cleaned := make([]any, 0, len(rules)+1)
	for _, rule := range rules {
		text, _ := rule.(string)
		if strings.HasPrefix(text, "MATCH,") {
			continue
		}
		cleaned = append(cleaned, rule)
	}
	config["rules"] = append(cleaned, "MATCH,"+target)
}

func processPaths(apps []model.LaunchApp) []string {
	paths := make([]string, 0, len(apps)+1)
	if executable, err := os.Executable(); err == nil {
		if absolute, err := filepath.Abs(executable); err == nil {
			paths = append(paths, absolute)
		}
	}
	for _, app := range apps {
		path := strings.TrimSpace(app.Path)
		if path == "" {
			continue
		}
		if absolute, err := filepath.Abs(path); err == nil {
			paths = append(paths, absolute)
		}
	}
	return paths
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

func yamlObject(parent map[string]any, key string) map[string]any {
	if object, ok := parent[key].(map[string]any); ok {
		return object
	}
	object := make(map[string]any)
	parent[key] = object
	return object
}

func validPort(port int) bool {
	return port >= 1 && port <= 65535
}
