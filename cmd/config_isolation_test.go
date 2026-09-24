package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/config"
)

// isolateHome points every default config lookup at a throwaway directory, so
// a command that saves its config cannot reach the user's real
// ~/.config/bookbeam/config.json on any platform.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.ConfigDirEnv, filepath.Join(home, ".config", "bookbeam"))
	return home
}

// testSettings resolves settings from a throwaway config.json holding host and
// token, with no environment or flag overrides, so a command that stores its
// token writes only into that file.
func testSettings(t *testing.T, host, token string) *config.Settings {
	t.Helper()
	return testSettingsIn(t, t.TempDir(), host, token)
}

// testSettingsIn is testSettings with config.json kept in dir.
func testSettingsIn(t *testing.T, dir, host, token string) *config.Settings {
	t.Helper()
	data, err := json.Marshal(config.Config{Host: host, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Resolve(dir, func(string) string { return "" }, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	return settings
}
