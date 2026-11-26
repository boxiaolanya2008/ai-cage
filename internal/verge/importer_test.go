package verge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

const runtimeConfig = `
port: 7899
socks-port: 7898
mixed-port: 7897
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9097
secret: set-your-secret
tun:
  enable: true
  device: Meta
dns:
  enable: true
proxies:
  - name: info-placeholder
    type: trojan
    server: 10.255.255.255
    port: 443
  - name: node
    type: ss
    server: example.com
    port: 8388
proxy-groups:
  - name: Proxy
    type: select
    proxies: ["info-placeholder", "node"]
rules:
  - MATCH,Proxy
proxy-providers:
  example:
    type: http
    url: https://example.com/provider
`

func TestSnapshotPrefersRuntimeAndSanitizesManagedFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, runtimeFile), []byte(runtimeConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := snapshotFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(snapshot, &config); err != nil {
		t.Fatal(err)
	}
	for _, key := range managedKeys {
		if _, exists := config[key]; exists {
			t.Errorf("%s was not removed", key)
		}
	}
	if _, exists := config["proxies"]; !exists {
		t.Error("proxies were removed")
	}
	if _, exists := config["proxy-groups"]; !exists {
		t.Error("proxy-groups were removed")
	}
	if _, exists := config["rules"]; !exists {
		t.Error("rules were removed")
	}
	if _, exists := config["proxy-providers"]; !exists {
		t.Error("proxy-providers were removed")
	}
	proxies := config["proxies"].([]any)
	for _, item := range proxies {
		proxy := item.(map[string]any)
		if proxy["server"] == "10.255.255.255" {
			t.Errorf("placeholder proxy was not removed: %#v", proxy)
		}
	}
	groups := config["proxy-groups"].([]any)
	group := groups[0].(map[string]any)
	if !reflect.DeepEqual(group["proxies"], []any{"node"}) {
		t.Errorf("placeholder proxy remains in group: %#v", group["proxies"])
	}
}

func TestSnapshotFallsBackToCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, profilesDir)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, profilesFile), []byte("current: current-profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "current-profile.yaml"), []byte(runtimeConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := snapshotFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) == 0 {
		t.Fatal("snapshot is empty")
	}
}

func TestSnapshotRejectsMissingCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	if _, err := snapshotFromDir(dir); err == nil {
		t.Fatal("missing Clash Verge data accepted")
	}
}

func TestSanitizeRejectsConfigWithoutProxies(t *testing.T) {
	if _, err := Sanitize([]byte("rules:\n  - MATCH,DIRECT\n")); err == nil {
		t.Fatal("config without proxies accepted")
	}
}
