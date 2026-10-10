package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/config"
)

// runRoot drives the real root command — flags, pre-run and subcommand — with
// config.json holding stored and env as the whole environment, and returns
// config.json afterwards.
func runRoot(t *testing.T, stored string, env map[string]string, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(stored), 0600); err != nil {
		t.Fatal(err)
	}
	getenv := func(key string) string {
		if key == config.ConfigDirEnv {
			return dir
		}
		return env[key]
	}
	noHome := func() (string, error) { return "", errors.New("home must not be consulted") }
	root := newRootCmd(getenv, noHome)
	root.SetArgs(append([]string{"--quiet"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Regression test for #42: one-off --host/--token/env overrides used to be
// merged into the struct that auth login/logout saved to config.json.
func TestOverridesAreNeverPersisted(t *testing.T) {
	stored := `{"host":"https://stored.example","token":"orig"}`
	cases := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{"env host on logout", map[string]string{"BOOKBEAM_HOST": "https://env.example"}, []string{"auth", "logout"},
			"{\n  \"host\": \"https://stored.example\"\n}"},
		{"env token on logout", map[string]string{"BOOKBEAM_TOKEN": "envtok"}, []string{"auth", "logout"},
			"{\n  \"host\": \"https://stored.example\"\n}"},
		{"flag host on login", nil, []string{"--host", "https://flag.example", "auth", "login", "--token", "newtok"},
			"{\n  \"host\": \"https://stored.example\",\n  \"token\": \"newtok\"\n}"},
		{"env host on login", map[string]string{"BOOKBEAM_HOST": "https://env.example"}, []string{"auth", "login", "--token", "newtok"},
			"{\n  \"host\": \"https://stored.example\",\n  \"token\": \"newtok\"\n}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runRoot(t, stored, tc.env, tc.args...); got != tc.want {
				t.Errorf("config.json after %v:\n got %s\nwant %s", tc.args, got, tc.want)
			}
		})
	}
}

func TestExistingConfigPermissionsEnforcedOnLoginAndLogout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"host":"https://bookbeam.app"}`), 0644); err != nil {
		t.Fatal(err)
	}
	getenv := func(key string) string {
		if key == config.ConfigDirEnv {
			return dir
		}
		return ""
	}
	noHome := func() (string, error) { return "", errors.New("home must not be consulted") }

	root := newRootCmd(getenv, noHome)
	root.SetArgs([]string{"auth", "login", "--token", "new-fake-token"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth login failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("file permissions after login = %o, want 600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"host\": \"https://bookbeam.app\",\n  \"token\": \"new-fake-token\"\n}"
	if string(data) != want {
		t.Errorf("config.json after login = %s, want %s", string(data), want)
	}

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	root = newRootCmd(getenv, noHome)
	root.SetArgs([]string{"auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth logout failed: %v", err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("file permissions after logout = %o, want 600", got)
	}
}

func TestLoginFailsWhenPersistenceFails(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	getenv := func(key string) string {
		if key == config.ConfigDirEnv {
			return filepath.Join(blocker, "sub")
		}
		return ""
	}
	noHome := func() (string, error) { return "", errors.New("home must not be consulted") }

	var buf bytes.Buffer
	root := newRootCmd(getenv, noHome)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"auth", "login", "--token", "fake-token", "--json"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected login to return an error when token cannot be saved")
	}
	if strings.Contains(buf.String(), "logged_in") {
		t.Errorf("expected no successful login output, got %s", buf.String())
	}
}

