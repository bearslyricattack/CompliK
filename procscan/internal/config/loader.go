package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"gopkg.in/yaml.v3"
)

// Loader handles configuration file loading and parsing
type Loader struct {
	configPath string
	lastHash   string
}

// NewLoader creates a new configuration loader
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// Load reads and parses the configuration file
func (l *Loader) Load() (*models.Config, error) {
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", l.configPath)
	}

	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file is empty: %s", l.configPath)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Update last hash
	hash, _ := l.calculateHash()
	l.lastHash = hash

	return &config, nil
}

// HasChanged checks if the configuration file has changed since last load
func (l *Loader) HasChanged() (bool, error) {
	currentHash, err := l.calculateHash()
	if err != nil {
		return false, err
	}

	if l.lastHash == "" {
		l.lastHash = currentHash
		return false, nil
	}

	changed := currentHash != l.lastHash
	if changed {
		l.lastHash = currentHash
	}

	return changed, nil
}

// GetConfigPath returns the configuration file path
func (l *Loader) GetConfigPath() string {
	return l.configPath
}

// GetConfigDir returns the directory containing the configuration file
func (l *Loader) GetConfigDir() string {
	return filepath.Dir(l.configPath)
}

// calculateHash computes SHA256 hash of the configuration file
func (l *Loader) calculateHash() (string, error) {
	file, err := os.Open(l.configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
