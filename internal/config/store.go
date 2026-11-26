package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boxiaolanya2008/ai-cage/internal/model"
)

const (
	settingsFile = "settings.json"
	appsFile     = "apps.json"
)

// Store persists application settings and launch apps in a data directory.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewDefaultStore creates a store in %APPDATA%/ai-cage.
func NewDefaultStore() (*Store, error) {
	appData := os.Getenv("APPDATA")
	if strings.TrimSpace(appData) == "" {
		return nil, errors.New("config: APPDATA is not set")
	}
	return NewStore(filepath.Join(appData, "ai-cage"))
}

// NewStore creates a store in dir and creates the directory if necessary.
func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("config: data directory is empty")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("config: resolve data directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return nil, fmt.Errorf("config: create data directory: %w", err)
	}
	return &Store{dir: absDir}, nil
}

// DataDir returns the directory used by the store.
func (s *Store) DataDir() string {
	return s.dir
}

// LoadSettings reads settings. A missing file yields the application defaults.
func (s *Store) LoadSettings() (model.Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := model.DefaultSettings()
	data, err := os.ReadFile(filepath.Join(s.dir, settingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return model.Settings{}, fmt.Errorf("config: read settings: %w", err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return model.Settings{}, fmt.Errorf("config: decode settings: %w", err)
	}
	return settings, nil
}

// SaveSettings atomically replaces settings.json.
func (s *Store) SaveSettings(settings model.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeJSON(filepath.Join(s.dir, settingsFile), settings); err != nil {
		return fmt.Errorf("config: save settings: %w", err)
	}
	return nil
}

// ListApps returns the configured launch apps. A missing apps.json is empty.
func (s *Store) ListApps() ([]model.LaunchApp, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apps, err := readApps(s.dir)
	if err != nil {
		return nil, fmt.Errorf("config: list apps: %w", err)
	}
	return apps, nil
}

// SaveApp adds or replaces the app with the same ID.
func (s *Store) SaveApp(app model.LaunchApp) error {
	if strings.TrimSpace(app.ID) == "" {
		return errors.New("config: app ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	apps, err := readApps(s.dir)
	if err != nil {
		return fmt.Errorf("config: save app: %w", err)
	}
	replaced := false
	for i := range apps {
		if apps[i].ID == app.ID {
			apps[i] = app
			replaced = true
			break
		}
	}
	if !replaced {
		apps = append(apps, app)
	}
	if err := writeJSON(filepath.Join(s.dir, appsFile), apps); err != nil {
		return fmt.Errorf("config: save app: %w", err)
	}
	return nil
}

// DeleteApp removes an app. Deleting an unknown ID is a no-op.
func (s *Store) DeleteApp(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("config: app ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	apps, err := readApps(s.dir)
	if err != nil {
		return fmt.Errorf("config: delete app: %w", err)
	}
	filtered := make([]model.LaunchApp, 0, len(apps))
	for _, app := range apps {
		if app.ID != id {
			filtered = append(filtered, app)
		}
	}
	if len(filtered) == len(apps) {
		return nil
	}
	if err := writeJSON(filepath.Join(s.dir, appsFile), filtered); err != nil {
		return fmt.Errorf("config: delete app: %w", err)
	}
	return nil
}

func readApps(dir string) ([]model.LaunchApp, error) {
	data, err := os.ReadFile(filepath.Join(dir, appsFile))
	if errors.Is(err, os.ErrNotExist) {
		return []model.LaunchApp{}, nil
	}
	if err != nil {
		return nil, err
	}
	var apps []model.LaunchApp
	if err := json.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("decode apps: %w", err)
	}
	if apps == nil {
		apps = []model.LaunchApp{}
	}
	return apps, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
