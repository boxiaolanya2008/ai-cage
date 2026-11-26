package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

func TestNewDefaultStoreAndLoadSettingsDefaults(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	store, err := NewDefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(appData, "ai-cage")
	if store.DataDir() != wantDir {
		t.Fatalf("DataDir() = %q, want %q", store.DataDir(), wantDir)
	}
	if _, err := os.Stat(wantDir); err != nil {
		t.Fatalf("data directory was not created: %v", err)
	}
	got, err := store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, model.DefaultSettings()) {
		t.Fatalf("defaults = %#v, want %#v", got, model.DefaultSettings())
	}
	if _, err := os.Stat(filepath.Join(wantDir, settingsFile)); !os.IsNotExist(err) {
		t.Fatalf("loading defaults should not create settings.json, err=%v", err)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "nested", "data"))
	if err != nil {
		t.Fatal(err)
	}
	want := model.DefaultSettings()
	want.KernelMode = model.KernelModeSidecar
	want.Sidecar.ActiveSubscription = "subscription-id"
	want.Sidecar.MihomoPath = `C:\tools\mihomo.exe`
	want.TUN.Enabled = true
	if err := store.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("settings = %#v, want %#v", got, want)
	}
}

func TestDamagedJSONReturnsErrors(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(`{"kernelMode":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadSettings(); err == nil {
		t.Fatal("LoadSettings accepted damaged JSON")
	}
	if err := os.WriteFile(filepath.Join(dir, appsFile), []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListApps(); err == nil {
		t.Fatal("ListApps accepted damaged JSON")
	}
}

func TestAppsCRUD(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	apps, err := store.ListApps()
	if err != nil || len(apps) != 0 {
		t.Fatalf("initial ListApps() = %#v, %v", apps, err)
	}
	first := model.LaunchApp{ID: "one", Name: "One", Path: `C:\one.exe`, Args: []string{"--first"}}
	second := model.LaunchApp{ID: "two", Name: "Two", Path: `C:\two.exe`}
	if err := store.SaveApp(first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveApp(second); err != nil {
		t.Fatal(err)
	}
	first.Args = []string{"--updated", "two words"}
	if err := store.SaveApp(first); err != nil {
		t.Fatal(err)
	}
	apps, err = store.ListApps()
	if err != nil {
		t.Fatal(err)
	}
	want := []model.LaunchApp{first, second}
	if !reflect.DeepEqual(apps, want) {
		t.Fatalf("ListApps() = %#v, want %#v", apps, want)
	}
	if err := store.DeleteApp("one"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteApp("one"); err != nil {
		t.Fatalf("deleting an unknown app should be idempotent: %v", err)
	}
	apps, err = store.ListApps()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(apps, []model.LaunchApp{second}) {
		t.Fatalf("apps after delete = %#v", apps)
	}
	if err := store.SaveApp(model.LaunchApp{}); err == nil {
		t.Fatal("SaveApp accepted an empty ID")
	}
	if err := store.DeleteApp(""); err == nil {
		t.Fatal("DeleteApp accepted an empty ID")
	}
}
