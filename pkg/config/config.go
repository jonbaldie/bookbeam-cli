package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	host   string
}

// Resolve reads config.json from dir and layers the environment and flag
// overrides over it.
func Resolve(dir string, getenv func(string) string, flags Overrides) (*Settings, error) {
	path := filepath.Join(dir, "config.json")
	stored, err := load(path)
	if err != nil {
		return nil, err
	}
	s := &Settings{
		path:   path,
		stored: *stored,
		env:    Overrides{Host: getenv(HostEnv), Token: getenv(TokenEnv)},
		flags:  flags,
	}
	raw, source := s.rawHost()
	if s.host, err = canonicalHost(raw); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", source, err)
	}
	return s, nil
}

// rawHost is the effective host as configured, with a description of the
// layer it came from for error messages.
func (s *Settings) rawHost() (string, string) {
	switch {
	case s.flags.Host != "":
		return s.flags.Host, fmt.Sprintf("--host %q", s.flags.Host)
	case s.env.Host != "":
		return s.env.Host, fmt.Sprintf("%s %q", HostEnv, s.env.Host)
	default:
		return s.stored.Host, fmt.Sprintf("host %q in %s", s.stored.Host, s.path)
	}
}

// canonicalHost checks that raw is an http or https base URL and returns it
// without trailing slashes.
func canonicalHost(raw string) (string, error) {
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "", errors.New("missing http:// or https://")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errors.New("missing host name")
	}
	return strings.TrimRight(raw, "/"), nil
}

// Host is the effective API host: an http or https base URL with no trailing
// slash.
func (s *Settings) Host() string {
	return s.host
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

	if err := os.Chmod(path, 0600); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
