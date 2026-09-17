package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Host  string `json:"host"`
	Token string `json:"token,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Host: "https://bookbeam.app",
	}
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bookbeam"), nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		var err error
		path, err = GetConfigPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
		}
	}

	if envHost := os.Getenv("BOOKBEAM_HOST"); envHost != "" {
		cfg.Host = envHost
	}
	if envToken := os.Getenv("BOOKBEAM_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	if path == "" {
		var err error
		path, err = GetConfigPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
