package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the on-disk shape of config.json: the stored layer of Settings.
type Config struct {
	Host  string `json:"host"`
	Token string `json:"token,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Host: "https://bookbeam.app",
	}
}

// ConfigDirEnv overrides the directory that holds config.json. It lets a
// caller — a test suite above all — point default config resolution somewhere
// other than the user's home directory.
const ConfigDirEnv = "BOOKBEAM_CONFIG_DIR"

// HostEnv and TokenEnv override the stored host and token for one invocation.
const (
	HostEnv  = "BOOKBEAM_HOST"
	TokenEnv = "BOOKBEAM_TOKEN"
)

// Dir resolves the directory that holds config.json from an environment
// lookup and a home-directory lookup, so callers decide where both come from.
func Dir(getenv func(string) string, home func() (string, error)) (string, error) {
	if dir := getenv(ConfigDirEnv); dir != "" {
		return dir, nil
	}
	homeDir, err := home()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bookbeam"), nil
}

// Overrides are a one-off host and token that win over the stored values
// without ever being written back to config.json.
type Overrides struct {
	Host  string
	Token string
}

// apply returns value unless the override is set.
func apply(value, override string) string {
	if override != "" {
		return override
	}
	return value
}

// Settings layers defaults < config file < environment < flags. Only the
// config-file layer is ever saved, so overrides never leak into config.json.
type Settings struct {
	path   string
	stored Config
	env    Overrides
	flags  Overrides
}

// Resolve reads config.json from dir and layers the environment and flag
// overrides over it.
func Resolve(dir string, getenv func(string) string, flags Overrides) (*Settings, error) {
	path := filepath.Join(dir, "config.json")
	stored, err := load(path)
	if err != nil {
		return nil, err
	}
	return &Settings{
		path:   path,
		stored: *stored,
		env:    Overrides{Host: getenv(HostEnv), Token: getenv(TokenEnv)},
		flags:  flags,
	}, nil
}

// Host is the effective API host.
func (s *Settings) Host() string {
	return apply(apply(s.stored.Host, s.env.Host), s.flags.Host)
}

// Token is the effective API token.
func (s *Settings) Token() string {
	return apply(apply(s.stored.Token, s.env.Token), s.flags.Token)
}

// StoreToken replaces the stored token and saves the config-file layer; an
// empty token clears it. Every other stored field keeps its on-disk value.
func (s *Settings) StoreToken(token string) error {
	s.stored.Token = token
	return save(&s.stored, s.path)
}

// load reads the config-file layer at path over the defaults; a missing file
// leaves the defaults in place.
func load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return cfg, nil
}

// save writes the config-file layer to path with owner-only permissions.
func save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
