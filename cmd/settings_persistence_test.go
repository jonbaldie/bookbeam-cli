package cmd

import (
	"errors"
	"os"
	"path/filepath"
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
