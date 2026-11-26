package mihomo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"gopkg.in/yaml.v3"
)

func TestMergeConfigPreservesSubscriptionAndOverridesManagedFields(t *testing.T) {
	raw := []byte(`
port: 7890
socks-port: 7891
redir-port: 7892
tproxy-port: 7893
mixed-port: 7890
allow-lan: true
mode: global
log-level: debug
external-controller: 0.0.0.0:9090
secret: old-secret
proxies:
  - name: "香港 / 01"
    type: ss
    server: example.com
    port: 8388
proxy-groups:
  - name: Proxy
    type: select
    proxies: ["香港 / 01"]
rules:
  - MATCH,Proxy
proxy-providers:
  example:
    type: http
    url: https://example.com/sub.yaml
    path: ./example.yaml
tun:
  enable: true
  mtu: 1400
dns:
  nameserver:
    - 9.9.9.9
  nameserver-policy:
    "+.example.com": 198.18.0.1
`)
	original := decodeConfigMap(t, raw)
	tun := model.TUNSettings{
		Enabled:             true,
		Stack:               "gvisor",
		DeviceName:          "ai-cage-test",
		AutoRoute:           true,
		StrictRoute:         true,
		AutoDetectInterface: true,
		IPv6:                true,
		PrimaryDNS:          "223.5.5.5",
		SecondaryDNS:        "119.29.29.29",
	}

	merged, err := MergeConfig(raw, 19097, 17897, "new-secret", tun, nil, "warning")
	if err != nil {
		t.Fatalf("MergeConfig() error = %v", err)
	}
	got := decodeConfigMap(t, merged)

	for _, key := range []string{"proxies", "rules", "proxy-providers"} {
		if !reflect.DeepEqual(got[key], original[key]) {
			t.Errorf("%s changed: got %#v, want %#v", key, got[key], original[key])
		}
	}
	groups := got["proxy-groups"].([]any)
	if len(groups) == 0 {
		t.Fatal("proxy-groups is empty")
	}
	first, ok := groups[0].(map[string]any)
	if !ok || first["name"] != aiCageGroup {
		t.Fatalf("first proxy group = %#v, want %s", groups[0], aiCageGroup)
	}
	if !reflect.DeepEqual(first["proxies"], []any{"Proxy"}) {
		t.Errorf("AI-CAGE upstreams = %#v, want [Proxy]", first["proxies"])
	}
	foundOriginal := false
	for _, group := range groups[1:] {
		if object, ok := group.(map[string]any); ok && object["name"] == "Proxy" {
			foundOriginal = true
			break
		}
	}
	if !foundOriginal {
		t.Error("original Proxy group was removed")
	}
	for key, want := range map[string]any{
		"port":                0,
		"socks-port":          0,
		"redir-port":          0,
		"tproxy-port":         0,
		"mixed-port":          17897,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "warning",
		"external-controller": "127.0.0.1:19097",
		"secret":              "new-secret",
		"ipv6":                true,
	} {
		if !reflect.DeepEqual(got[key], want) {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}

	tunConfig := nestedMap(t, got, "tun")
	for key, want := range map[string]any{
		"enable":                true,
		"device":                "ai-cage-test",
		"stack":                 "gvisor",
		"auto-route":            true,
		"strict-route":          true,
		"auto-detect-interface": true,
		"dns-hijack":            []any{"any:53"},
		"mtu":                   1400,
	} {
		if !reflect.DeepEqual(tunConfig[key], want) {
			t.Errorf("tun.%s = %#v, want %#v", key, tunConfig[key], want)
		}
	}

	dns := nestedMap(t, got, "dns")
	for key, want := range map[string]any{
		"enable":            true,
		"enhanced-mode":     "fake-ip",
		"fake-ip-range":     "198.18.0.1/16",
		"nameserver":        []any{"223.5.5.5", "119.29.29.29"},
		"nameserver-policy": original["dns"].(map[string]any)["nameserver-policy"],
	} {
		if !reflect.DeepEqual(dns[key], want) {
			t.Errorf("dns.%s = %#v, want %#v", key, dns[key], want)
		}
	}
	if _, exists := got["dns-hijack"]; exists {
		t.Error("dns-hijack must be nested under tun, not at the top level")
	}
}

func TestMergeConfigExplicitlyDisablesTUN(t *testing.T) {
	merged, err := MergeConfig([]byte(`
tun:
  enable: true
  device: old-device
  dns-hijack:
    - any:53
dns-hijack:
  - any:53
`), 9097, 7890, "", model.TUNSettings{
		PrimaryDNS:   "1.1.1.1",
		SecondaryDNS: "8.8.8.8",
	}, nil, "info")
	if err != nil {
		t.Fatalf("MergeConfig() error = %v", err)
	}
	config := decodeConfigMap(t, merged)
	if got := nestedMap(t, config, "tun")["enable"]; got != false {
		t.Errorf("tun.enable = %#v, want false", got)
	}
	if got := nestedMap(t, config, "tun")["device"]; got != "old-device" {
		t.Errorf("unmanaged tun.device = %#v, want old-device", got)
	}
	if got := nestedMap(t, config, "tun")["dns-hijack"]; !reflect.DeepEqual(got, []any{}) {
		t.Errorf("tun.dns-hijack = %#v, want empty list", got)
	}
	if _, exists := config["dns-hijack"]; exists {
		t.Error("obsolete top-level dns-hijack must be removed")
	}
}

func TestMergeConfigRejectsInvalidInput(t *testing.T) {
	tun := model.TUNSettings{PrimaryDNS: "1.1.1.1", SecondaryDNS: "8.8.8.8"}
	tests := []struct {
		name           string
		raw            string
		controllerPort int
		mixedPort      int
		tun            model.TUNSettings
	}{
		{name: "malformed yaml", raw: "proxies: [", controllerPort: 9097, mixedPort: 7890, tun: tun},
		{name: "top-level list", raw: "- proxies", controllerPort: 9097, mixedPort: 7890, tun: tun},
		{name: "zero controller port", raw: "{}", controllerPort: 0, mixedPort: 7890, tun: tun},
		{name: "oversized mixed port", raw: "{}", controllerPort: 9097, mixedPort: 65536, tun: tun},
		{name: "missing primary DNS", raw: "{}", controllerPort: 9097, mixedPort: 7890, tun: model.TUNSettings{SecondaryDNS: "8.8.8.8"}},
		{name: "missing secondary DNS", raw: "{}", controllerPort: 9097, mixedPort: 7890, tun: model.TUNSettings{PrimaryDNS: "1.1.1.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := MergeConfig([]byte(tt.raw), tt.controllerPort, tt.mixedPort, "secret", tt.tun, nil, "info"); err == nil {
				t.Fatal("MergeConfig() error = nil, want an error")
			}
		})
	}
}

func TestMergeConfigAcceptsEmptyDocument(t *testing.T) {
	merged, err := MergeConfig(nil, 9097, 7890, "secret", model.TUNSettings{
		PrimaryDNS:   "1.1.1.1",
		SecondaryDNS: "8.8.8.8",
	}, nil, "silent")
	if err != nil {
		t.Fatalf("MergeConfig() error = %v", err)
	}
	if !strings.Contains(string(merged), "mixed-port: 7890") {
		t.Fatalf("merged config does not contain managed mixed port: %s", merged)
	}
}

func TestMergeConfigAddsCagedProcessRules(t *testing.T) {
	raw := []byte(`
proxies:
  - name: node
    type: ss
    server: example.com
    port: 8388
proxy-groups:
  - name: Proxy
    type: select
    proxies: ["node"]
rules:
  - MATCH,Proxy
`)
	app := model.LaunchApp{ID: "app", Name: "App", Path: `C:\Apps\caged.exe`}
	tun := model.TUNSettings{
		Enabled:             true,
		OnlyCagedApps:       true,
		PrimaryDNS:          "223.5.5.5",
		SecondaryDNS:        "119.29.29.29",
		Stack:               "gvisor",
		DeviceName:          "ai-cage",
		AutoRoute:           true,
		StrictRoute:         true,
		AutoDetectInterface: true,
	}

	merged, err := MergeConfig(raw, 19097, 17897, "secret", tun, []model.LaunchApp{app}, "info")
	if err != nil {
		t.Fatalf("MergeConfig() error = %v", err)
	}
	config := decodeConfigMap(t, merged)
	if config["find-process-mode"] != "always" {
		t.Errorf("find-process-mode = %#v, want always", config["find-process-mode"])
	}

	rules := config["rules"].([]any)
	if len(rules) < 3 {
		t.Fatalf("rules = %#v, want process rules and MATCH,DIRECT", rules)
	}
	processFound := false
	for _, rule := range rules[:len(rules)-1] {
		text, _ := rule.(string)
		if strings.Contains(text, `PROCESS-PATH,C:\Apps\caged.exe,`+aiCageGroup) {
			processFound = true
		}
	}
	if !processFound {
		t.Errorf("caged app process rule missing: %#v", rules)
	}
	for _, rule := range rules[:len(rules)-1] {
		if rule == "MATCH,Proxy" {
			t.Errorf("original final MATCH was not replaced: %#v", rules)
		}
	}
	if rules[len(rules)-1] != "MATCH,DIRECT" {
		t.Errorf("fallback rule = %#v, want MATCH,DIRECT", rules[len(rules)-1])
	}

	groups := config["proxy-groups"].([]any)
	first := groups[0].(map[string]any)
	if first["name"] != aiCageGroup || !reflect.DeepEqual(first["proxies"], []any{"Proxy"}) {
		t.Errorf("AI-CAGE group = %#v, want upstream Proxy", first)
	}
}

func TestMergeConfigKeepsOriginalRulesWhenCageDisabled(t *testing.T) {
	raw := []byte("rules:\n  - MATCH,Proxy\n")
	merged, err := MergeConfig(raw, 19097, 17897, "secret", model.TUNSettings{
		PrimaryDNS:   "1.1.1.1",
		SecondaryDNS: "8.8.8.8",
	}, nil, "info")
	if err != nil {
		t.Fatalf("MergeConfig() error = %v", err)
	}
	config := decodeConfigMap(t, merged)
	if !reflect.DeepEqual(config["rules"], []any{"MATCH,Proxy"}) {
		t.Errorf("rules = %#v, want original rules only", config["rules"])
	}
	if config["find-process-mode"] != "strict" {
		t.Errorf("find-process-mode = %#v, want strict", config["find-process-mode"])
	}
}

func decodeConfigMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if config == nil {
		t.Fatal("decoded config is nil")
	}
	return config
}

func nestedMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	object, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s has type %T, want map[string]any", key, parent[key])
	}
	return object
}
