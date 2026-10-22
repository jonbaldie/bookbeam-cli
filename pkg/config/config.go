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
// with a lower-case scheme and no trailing slashes.
func canonicalHost(raw string) (string, error) {
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "", errors.New("must start with http:// or https://")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.Unwrap(err)
	}
	switch {
	case u.Hostname() == "":
		return "", errors.New("missing host name")
	case u.User != nil:
		return "", errors.New("unexpected credentials")
	case u.ForceQuery || u.RawQuery != "" || u.Fragment != "":
		return "", errors.New("unexpected query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u.String(), nil
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
// It is all-or-nothing: on error, neither Token nor config.json has changed.
func (s *Settings) StoreToken(token string) error {
	candidate := s.stored
	candidate.Token = token
	if err := save(&candidate, s.path); err != nil {
		return err
	}
	s.stored = candidate
	return nil
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

// configFile is the part of *os.File that save writes a candidate through.
type configFile interface {
	Write(data []byte) (int, error)
	Close() error
	Name() string
}

// createTemp opens a private (0600) temporary file in dir. Tests replace it
// to inject write and close failures.
var createTemp = func(dir, pattern string) (configFile, error) {
	return os.CreateTemp(dir, pattern)
}

// save commits the config-file layer to path with owner-only permissions. It
// writes a complete temporary file beside path and renames it over path, so a
// failure leaves any previous config.json untouched and no temporary file
// behind. The rename replaces config.json on every platform; it is not synced
// to disk, so it does not promise durability across power loss.
func save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := createTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return err
	}
	if err := replace(tmp, data, path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// replace writes data to tmp and closes it, then renames it over path only
// once both have succeeded.
func replace(tmp configFile, data []byte, path string) error {
	_, err := tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
