package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestLoadMissingConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "nonexistent.json")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected missing config to be tolerated, got error: %v", err)
	}
	if cfg.Host != "https://bookbeam.app" {
		t.Errorf("expected default host, got %s", cfg.Host)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token, got %s", cfg.Token)
	}
}

func TestLoadUnreadableConfigFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions not applicable on Windows")
	}

	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"token":"secret"}`), 0000); err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected error loading unreadable config file, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, configPath) {
		t.Errorf("expected error to mention config path %q, got: %s", configPath, errMsg)
	}
	if !strings.Contains(errMsg, "permission denied") {
		t.Errorf("expected error to mention permission denied, got: %s", errMsg)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("expected wrapped fs.ErrPermission, got: %v", err)
	}
}

func TestLoadInvalidJSONConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{not-valid-json`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected error loading invalid JSON config file, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, configPath) {
		t.Errorf("expected error to mention config path %q, got: %s", configPath, errMsg)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("expected wrapped json.SyntaxError, got: %v", err)
	}
}

func TestLoadEmptyConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bookbeam-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(``), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected error loading empty config file, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, configPath) {
		t.Errorf("expected error to mention config path %q, got: %s", configPath, errMsg)
	}
}

func TestGetConfigDirAndPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir failed: %v", err)
	}
	expectedDir := filepath.Join(tempDir, ".config", "bookbeam")
	if dir != expectedDir {
		t.Errorf("expected dir %q, got %q", expectedDir, dir)
	}

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}
	expectedPath := filepath.Join(expectedDir, "config.json")
	if path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, path)
	}
}

func TestLoadAndSaveEmptyPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	defer os.Remove("config.json")

	cfg := &Config{
		Host:  "http://empty-path.test",
		Token: "token-empty-path",
	}
	if err := Save(cfg, ""); err != nil {
		t.Fatalf("Save with empty path failed: %v", err)
	}

	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load with empty path failed: %v", err)
	}
	if loaded.Host != cfg.Host {
		t.Errorf("expected host %q, got %q", cfg.Host, loaded.Host)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("expected token %q, got %q", cfg.Token, loaded.Token)
	}
}

func TestSaveMkdirError(t *testing.T) {
	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("blocker"), 0600); err != nil {
		t.Fatal(err)
	}

	invalidPath := filepath.Join(blockingFile, "sub", "config.json")
	cfg := &Config{Host: "https://example.com"}
	err := Save(cfg, invalidPath)
	if err == nil {
		t.Fatal("expected error from Save when MkdirAll fails, got nil")
	}
}
