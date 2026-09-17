package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func unsetHome(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("home directory lookup does not use $HOME on this platform")
	}
	t.Setenv("HOME", "")
}

func TestConfigPathsFailWithoutHomeDirectory(t *testing.T) {
	unsetHome(t)

	if dir, err := GetConfigDir(); err == nil {
		t.Fatalf("expected GetConfigDir error, got %q", dir)
	}
	if path, err := GetConfigPath(); err == nil {
		t.Fatalf("expected GetConfigPath error, got %q", path)
	}
	if _, err := Load(""); err == nil {
		t.Fatal("expected Load error without a home directory")
	}
}

func TestSaveWithoutHomeDirectoryWritesNothing(t *testing.T) {
	unsetHome(t)
	workDir := t.TempDir()
	t.Chdir(workDir)

	err := Save(&Config{Host: "https://example.com"}, "")
	if err == nil || !strings.Contains(err.Error(), "$HOME") {
		t.Fatalf("expected home directory error from Save, got %v", err)
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected nothing written to the working directory, found %d entries", len(entries))
	}
}

func TestSaveReportsDirectoryCreationFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	err := Save(&Config{Host: "https://example.com"}, filepath.Join(blocker, "config.json"))
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "mkdir" {
		t.Fatalf("expected mkdir path error, got %v", err)
	}
}

func TestSaveCreatesPrivateDirectoryAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := filepath.Join(t.TempDir(), "nested", "bookbeam")
	path := filepath.Join(dir, "config.json")

	if err := Save(&Config{Host: "https://example.com", Token: "t"}, path); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Errorf("directory permissions = %o, want 700", got)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Errorf("file permissions = %o, want 600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"host\": \"https://example.com\",\n  \"token\": \"t\"\n}"
	if string(data) != want {
		t.Errorf("saved config = %q, want %q", data, want)
	}
}
