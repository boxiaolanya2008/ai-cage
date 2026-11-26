package subscription

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boxiaolanya2008/ai-cage/internal/config"
	"github.com/boxiaolanya2008/ai-cage/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	RequestTimeout   = 20 * time.Second
	MaxContentSize   = 8 << 20
	MaxRedirects     = 5
	subscriptionsDir = "subscriptions"
	indexFilename    = "subscriptions.json"
)

// ErrNotFound indicates that a subscription ID is not registered.
var ErrNotFound = errors.New("subscription not found")

// Manager downloads, validates, and persists subscriptions.
type Manager struct {
	store  *config.Store
	client *http.Client

	operationMu sync.Mutex
	indexMu     sync.RWMutex
}

// NewManager creates a manager backed by the store's data directory.
func NewManager(store *config.Store) (*Manager, error) {
	if store == nil {
		return nil, errors.New("subscription: config store is nil")
	}
	if strings.TrimSpace(store.DataDir()) == "" {
		return nil, errors.New("subscription: config data directory is empty")
	}
	dir := filepath.Join(store.DataDir(), subscriptionsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("subscription: create directory: %w", err)
	}
	return &Manager{
		store: store,
		client: &http.Client{
			Timeout: RequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= MaxRedirects {
					return fmt.Errorf("subscription: stopped after %d redirects", MaxRedirects)
				}
				if _, err := normalizeURL(req.URL.String()); err != nil {
					return fmt.Errorf("subscription: invalid redirect URL: %w", err)
				}
				return nil
			},
		},
	}, nil
}

// List returns all registered subscriptions.
func (m *Manager) List() ([]model.Subscription, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	items, err := readIndex(m.store.DataDir())
	if err != nil {
		return nil, fmt.Errorf("subscription: list: %w", err)
	}
	return items, nil
}

// Add downloads and registers a subscription with a cryptographically random ID.
func (m *Manager) Add(name, rawURL string) (model.Subscription, error) {
	if err := m.validate(); err != nil {
		return model.Subscription{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Subscription{}, errors.New("subscription: name is required")
	}
	subscriptionURL, err := normalizeURL(rawURL)
	if err != nil {
		return model.Subscription{}, err
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	data, err := m.fetch(subscriptionURL)
	if err != nil {
		return model.Subscription{}, err
	}
	id, err := newID()
	if err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: generate ID: %w", err)
	}
	item := model.Subscription{
		ID: id, Name: name, URL: subscriptionURL, Kind: model.SubscriptionKindRemote, UpdatedAt: time.Now().UTC(),
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	if err := os.MkdirAll(m.contentDir(), 0o700); err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: create directory: %w", err)
	}
	if err := writeFileAtomic(m.contentPath(id), data); err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: save config: %w", err)
	}
	items, err := readIndex(m.store.DataDir())
	if err != nil {
		_ = os.Remove(m.contentPath(id))
		return model.Subscription{}, fmt.Errorf("subscription: add: %w", err)
	}
	items = append(items, item)
	if err := writeIndex(m.store.DataDir(), items); err != nil {
		_ = os.Remove(m.contentPath(id))
		return model.Subscription{}, fmt.Errorf("subscription: add: %w", err)
	}
	return item, nil
}

// Update refreshes the content and timestamp of an existing subscription.
func (m *Manager) Update(id string) (model.Subscription, error) {
	if err := m.validate(); err != nil {
		return model.Subscription{}, err
	}
	if err := validateID(id); err != nil {
		return model.Subscription{}, err
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	item, err := m.find(id)
	if err != nil {
		return model.Subscription{}, err
	}
	if item.Kind == model.SubscriptionKindVerge {
		return model.Subscription{}, errors.New("Verge 快照请重新导入")
	}
	data, err := m.fetch(item.URL)
	if err != nil {
		return model.Subscription{}, err
	}
	item.UpdatedAt = time.Now().UTC()

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	items, err := readIndex(m.store.DataDir())
	if err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: update: %w", err)
	}
	position := indexOf(items, id)
	if position < 0 {
		return model.Subscription{}, fmt.Errorf("subscription: update: %w", ErrNotFound)
	}
	rollback, err := m.replaceContent(id, data)
	if err != nil {
		return model.Subscription{}, err
	}
	items[position] = item
	if err := writeIndex(m.store.DataDir(), items); err != nil {
		if rollback != nil {
			_ = rollback()
		}
		return model.Subscription{}, fmt.Errorf("subscription: update: %w", err)
	}
	return item, nil
}

// Delete removes a subscription and its YAML file. Unknown IDs return ErrNotFound.
func (m *Manager) Delete(id string) error {
	if err := m.validate(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	items, err := readIndex(m.store.DataDir())
	if err != nil {
		return fmt.Errorf("subscription: delete: %w", err)
	}
	position := indexOf(items, id)
	if position < 0 {
		return fmt.Errorf("subscription: delete: %w", ErrNotFound)
	}

	path := m.contentPath(id)
	oldData, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("subscription: delete: read config: %w", readErr)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("subscription: delete config: %w", err)
	}

	items = append(items[:position], items[position+1:]...)
	if err := writeIndex(m.store.DataDir(), items); err != nil {
		if readErr == nil {
			_ = writeFileAtomic(path, oldData)
		}
		return fmt.Errorf("subscription: delete: %w", err)
	}
	return nil
}

// ImportSnapshot stores a local configuration snapshot. Repeated imports
// replace the existing snapshot instead of creating duplicates.
func (m *Manager) ImportSnapshot(name string, data []byte) (model.Subscription, error) {
	if err := m.validate(); err != nil {
		return model.Subscription{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Subscription{}, errors.New("subscription: snapshot name is required")
	}
	if err := validateYAML(data); err != nil {
		return model.Subscription{}, err
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	items, err := readIndex(m.store.DataDir())
	if err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: import: %w", err)
	}
	if err := os.MkdirAll(m.contentDir(), 0o700); err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: create directory: %w", err)
	}

	now := time.Now().UTC()
	for index := range items {
		if items[index].Kind == model.SubscriptionKindVerge && items[index].URL == "verge://current" {
			item := items[index]
			item.Name = name
			item.UpdatedAt = now
			if err := writeFileAtomic(m.contentPath(item.ID), data); err != nil {
				return model.Subscription{}, fmt.Errorf("subscription: import config: %w", err)
			}
			items[index] = item
			if err := writeIndex(m.store.DataDir(), items); err != nil {
				return model.Subscription{}, fmt.Errorf("subscription: import: %w", err)
			}
			return item, nil
		}
	}

	id, err := newID()
	if err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: generate ID: %w", err)
	}
	item := model.Subscription{
		ID:        id,
		Name:      name,
		URL:       "verge://current",
		Kind:      model.SubscriptionKindVerge,
		UpdatedAt: now,
	}
	if err := writeFileAtomic(m.contentPath(id), data); err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: import config: %w", err)
	}
	items = append(items, item)
	if err := writeIndex(m.store.DataDir(), items); err != nil {
		_ = os.Remove(m.contentPath(id))
		return model.Subscription{}, fmt.Errorf("subscription: import: %w", err)
	}
	return item, nil
}

// ReadConfig returns the raw YAML for a registered subscription.
func (m *Manager) ReadConfig(id string) ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	if err := validateID(id); err != nil {
		return nil, err
	}
	if _, err := m.find(id); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(m.contentPath(id))
	if err != nil {
		return nil, fmt.Errorf("subscription: read config: %w", err)
	}
	return data, nil
}

func (m *Manager) validate() error {
	if m == nil || m.store == nil || m.client == nil {
		return errors.New("subscription: manager is not initialized")
	}
	return nil
}

func (m *Manager) find(id string) (model.Subscription, error) {
	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	items, err := readIndex(m.store.DataDir())
	if err != nil {
		return model.Subscription{}, fmt.Errorf("subscription: find: %w", err)
	}
	position := indexOf(items, id)
	if position < 0 {
		return model.Subscription{}, fmt.Errorf("subscription: find: %w", ErrNotFound)
	}
	return items[position], nil
}

func (m *Manager) replaceContent(id string, data []byte) (func() error, error) {
	path := m.contentPath(id)
	oldData, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, fmt.Errorf("subscription: update: read old config: %w", readErr)
	}
	if err := writeFileAtomic(path, data); err != nil {
		return nil, fmt.Errorf("subscription: update config: %w", err)
	}
	return func() error {
		if readErr == nil {
			return writeFileAtomic(path, oldData)
		}
		return os.Remove(path)
	}, nil
}

func (m *Manager) contentDir() string {
	return filepath.Join(m.store.DataDir(), subscriptionsDir)
}

func (m *Manager) contentPath(id string) string {
	return filepath.Join(m.contentDir(), id+".yaml")
}

func (m *Manager) fetch(rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("subscription: create request: %w", err)
	}
	req.Header.Set("User-Agent", "ai-cage")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("subscription: download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subscription: download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxContentSize {
		return nil, fmt.Errorf("subscription: config exceeds %d bytes", MaxContentSize)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxContentSize+1))
	if err != nil {
		return nil, fmt.Errorf("subscription: read response: %w", err)
	}
	if len(data) > MaxContentSize {
		return nil, fmt.Errorf("subscription: config exceeds %d bytes", MaxContentSize)
	}
	if err := validateYAML(data); err != nil {
		return nil, err
	}
	return data, nil
}

func normalizeURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("subscription: invalid URL: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("subscription: URL must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("subscription: URL must include a host")
	}
	return parsed.String(), nil
}

func validateYAML(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("subscription: invalid YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("subscription: invalid YAML: %w", err)
		}
		return errors.New("subscription: YAML must contain a single document")
	}
	for _, key := range []string{"proxies", "proxy-providers"} {
		if value, ok := document[key]; ok && hasEntries(value) {
			return nil
		}
	}
	return errors.New("subscription: YAML must contain non-empty proxies or proxy-providers")
}

func hasEntries(value any) bool {
	switch typed := value.(type) {
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return false
	}
}

func readIndex(dataDir string) ([]model.Subscription, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, indexFilename))
	if errors.Is(err, os.ErrNotExist) {
		return []model.Subscription{}, nil
	}
	if err != nil {
		return nil, err
	}
	var items []model.Subscription
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode subscription index: %w", err)
	}
	if items == nil {
		items = []model.Subscription{}
	}
	for index := range items {
		if items[index].Kind == "" {
			items[index].Kind = model.SubscriptionKindRemote
		}
	}
	return items, nil
}

func writeIndex(dataDir string, items []model.Subscription) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(filepath.Join(dataDir, indexFilename), data)
}

func indexOf(items []model.Subscription, id string) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validateID(id string) error {
	if len(id) != 32 {
		return errors.New("subscription: invalid ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return fmt.Errorf("subscription: invalid ID: %w", err)
	}
	return nil
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
