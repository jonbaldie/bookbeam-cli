package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "https://bookbeam.app" {
		t.Fatalf("expected host https://bookbeam.app, got %s", cfg.Host)
	}
	if cfg.Token != "" {
		t.Fatalf("expected empty token, got %s", cfg.Token)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	initialCfg := &Config{
		Host:  "http://localhost:8000",
		Token: "test-token-xyz",
	}

	if err := Save(initialCfg, configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	// Windows does not expose POSIX file permissions through os.FileMode.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0600 {
		t.Fatalf("expected 0600 permissions, got %o", perm)
	}

	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loadedCfg.Host != "http://localhost:8000" {
		t.Fatalf("expected host http://localhost:8000, got %s", loadedCfg.Host)
	}
	if loadedCfg.Token != "test-token-xyz" {
		t.Fatalf("expected token test-token-xyz, got %s", loadedCfg.Token)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	os.Setenv("BOOKBEAM_HOST", "http://custom-host.test")
	os.Setenv("BOOKBEAM_TOKEN", "env-token-123")
	defer func() {
		os.Unsetenv("BOOKBEAM_HOST")
		os.Unsetenv("BOOKBEAM_TOKEN")
	}()

	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Host != "http://custom-host.test" {
		t.Fatalf("expected host from env http://custom-host.test, got %s", cfg.Host)
	}
	if cfg.Token != "env-token-123" {
		t.Fatalf("expected token from env env-token-123, got %s", cfg.Token)
	}
}
