package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// realHomeDir is the developer's actual home directory, captured before the
// sandbox replaces it. Tests assert that config resolution stays out of it.
var realHomeDir string

// TestMain sandboxes every test in this package: default config resolution is
// pointed at a throwaway directory, and the home lookup is stubbed, so no test
// — and no mutant of this package — can write to the real ~/.config/bookbeam.
func TestMain(m *testing.M) {
	realHomeDir, _ = os.UserHomeDir()

	sandbox, err := os.MkdirTemp("", "bookbeam-config-sandbox-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", sandbox)
	os.Setenv("USERPROFILE", sandbox)
	os.Setenv(ConfigDirEnv, filepath.Join(sandbox, ".config", "bookbeam"))
	os.Unsetenv("BOOKBEAM_HOST")
	os.Unsetenv("BOOKBEAM_TOKEN")
	userHomeDir = func() (string, error) { return sandbox, nil }

	code := m.Run()
	os.RemoveAll(sandbox)
	os.Exit(code)
}

// isolateConfigDir points default config resolution at a fresh temp directory
// for one test and returns it. The working directory moves to a temp directory
// too, so a relative fallback path cannot litter the package directory.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bookbeam")
	t.Setenv(ConfigDirEnv, dir)
	t.Chdir(t.TempDir())
	return dir
}

// stubHomeDir replaces the home lookup for one test.
func stubHomeDir(t *testing.T, home string, err error) {
	t.Helper()
	previous := userHomeDir
	userHomeDir = func() (string, error) { return home, err }
	t.Cleanup(func() { userHomeDir = previous })
}

// failHomeLookup makes default config resolution fail the way it does on a
// machine with no home directory, without touching process-global HOME.
func failHomeLookup(t *testing.T) {
	t.Helper()
	t.Setenv(ConfigDirEnv, "")
	t.Chdir(t.TempDir())
	stubHomeDir(t, "", errors.New("$HOME is not defined"))
}
