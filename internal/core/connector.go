package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const connectorFile = "core.json"

func connectorPath() (string, error) {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve local application data: %w", err)
		}
	}
	return filepath.Join(base, "ai-cage", connectorFile), nil
}

func newToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate core token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func writeConnector(connector Connector) error {
	path, err := connectorPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(connector)
	if err != nil {
		return fmt.Errorf("encode core connector: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("write core connector: %w", err)
	}
	return nil
}

// ReadConnector reads the connector published by the elevated core.
func ReadConnector() (Connector, error) {
	path, err := connectorPath()
	if err != nil {
		return Connector{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Connector{}, errors.New("core connector is not available")
	}
	if err != nil {
		return Connector{}, fmt.Errorf("read core connector: %w", err)
	}
	var connector Connector
	if err := json.Unmarshal(data, &connector); err != nil {
		return Connector{}, fmt.Errorf("decode core connector: %w", err)
	}
	if connector.Endpoint == "" || connector.Token == "" {
		return Connector{}, errors.New("core connector is incomplete")
	}
	return connector, nil
}

func removeConnector(connector Connector) error {
	path, err := connectorPath()
	if err != nil {
		return err
	}
	current, err := ReadConnector()
	if err != nil || current.Endpoint != connector.Endpoint || current.Token != connector.Token {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove core connector: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".core-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}()
	if err := file.Chmod(mode); err != nil {
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
