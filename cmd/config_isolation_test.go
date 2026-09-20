package cmd

import (
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
