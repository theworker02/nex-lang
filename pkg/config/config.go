// Package config loads and stores nex CLI credentials and registry settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// DirName is the config directory under the user config root.
	DirName = "nex"
	// FileName is the credentials/config file name.
	FileName = "config.toml"
	// EnvRegistryURL overrides the configured registry base URL.
	EnvRegistryURL = "NEX_REGISTRY_URL"
	// EnvToken is an API key or session token (preferred env name).
	EnvToken = "NEX_TOKEN"
	// EnvAPIKey is an alias for EnvToken (cargo/npm style).
	EnvAPIKey = "NEX_API_KEY"
	// DefaultRegistryURL matches the local nex-registry default.
	DefaultRegistryURL = "http://localhost:8080"
)

// Config is the on-disk nex CLI configuration.
type Config struct {
	RegistryURL string `toml:"registry_url"`
	Token       string `toml:"token"`
	Username    string `toml:"username,omitempty"`
}

// Path returns the absolute path to the user config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, DirName, FileName), nil
}

// Dir returns the nex config directory.
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, DirName), nil
}

// Load reads the config file if present. Missing file yields defaults (empty token).
func Load() (*Config, error) {
	cfg := &Config{RegistryURL: DefaultRegistryURL}
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.RegistryURL) == "" {
		cfg.RegistryURL = DefaultRegistryURL
	}
	applyEnv(cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv(EnvRegistryURL)); v != "" {
		cfg.RegistryURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv(EnvToken)); v != "" {
		cfg.Token = v
		return
	}
	if v := strings.TrimSpace(os.Getenv(EnvAPIKey)); v != "" {
		cfg.Token = v
	}
}

// Save writes the config to disk (0600 on Unix; best-effort on Windows).
func (c *Config) Save() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path, err := Path()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// ClearCredentials removes the stored token and username, keeping registry_url.
func (c *Config) ClearCredentials() {
	if c == nil {
		return
	}
	c.Token = ""
	c.Username = ""
}

// RegistryBase returns the trimmed registry URL with env overrides applied.
func (c *Config) RegistryBase() string {
	if c == nil {
		return DefaultRegistryURL
	}
	u := strings.TrimSpace(c.RegistryURL)
	if u == "" {
		u = DefaultRegistryURL
	}
	return strings.TrimRight(u, "/")
}

// AuthToken returns the bearer token from env or config (env wins via Load).
func (c *Config) AuthToken() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Token)
}
