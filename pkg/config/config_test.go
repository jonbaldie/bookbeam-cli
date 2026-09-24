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

// envOf is an injected environment lookup backed by vars.
func envOf(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// writeConfig writes body as config.json in a fresh directory and returns it.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "https://bookbeam.app" {
		t.Fatalf("expected host https://bookbeam.app, got %s", cfg.Host)
	}
	if cfg.Token != "" {
		t.Fatalf("expected empty token, got %s", cfg.Token)
	}
}

func TestDir(t *testing.T) {
	home := func() (string, error) { return "/home/ann", nil }
	noHome := func() (string, error) { return "", errors.New("$HOME is not defined") }

	dir, err := Dir(envOf(nil), home)
	if err != nil || dir != filepath.Join("/home/ann", ".config", "bookbeam") {
		t.Errorf("home dir = %q, %v", dir, err)
	}
	dir, err = Dir(envOf(map[string]string{ConfigDirEnv: "/elsewhere"}), noHome)
	if err != nil || dir != "/elsewhere" {
		t.Errorf("override dir = %q, %v", dir, err)
	}
	dir, err = Dir(envOf(nil), noHome)
	if err == nil || err.Error() != "$HOME is not defined" || dir != "" {
		t.Errorf("no home = %q, %v", dir, err)
	}
}

// Precedence at the settings interface: defaults < file < env < flags.
func TestResolvePrecedence(t *testing.T) {
	const file = `{"host":"https://file.example","token":"file-token"}`
	env := map[string]string{HostEnv: "https://env.example", TokenEnv: "env-token"}
	flags := Overrides{Host: "https://flag.example", Token: "flag-token"}
	cases := []struct {
		name      string
		file      string
		env       map[string]string
		flags     Overrides
		host, tok string
	}{
		{"defaults", "", nil, Overrides{}, "https://bookbeam.app", ""},
		{"file over defaults", file, nil, Overrides{}, "https://file.example", "file-token"},
		{"env over file", file, env, Overrides{}, "https://env.example", "env-token"},
		{"env over defaults", "", env, Overrides{}, "https://env.example", "env-token"},
		{"flags over env", file, env, flags, "https://flag.example", "flag-token"},
		{"flags over file", file, nil, flags, "https://flag.example", "flag-token"},
		{"env host only", file, map[string]string{HostEnv: "https://env.example"}, Overrides{}, "https://env.example", "file-token"},
		{"env token only", file, map[string]string{TokenEnv: "env-token"}, Overrides{}, "https://file.example", "env-token"},
		{"flag host only", file, env, Overrides{Host: "https://flag.example"}, "https://flag.example", "env-token"},
		{"flag token only", file, env, Overrides{Token: "flag-token"}, "https://env.example", "flag-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				dir = writeConfig(t, tc.file)
			}
			s, err := Resolve(dir, envOf(tc.env), tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			if s.Host() != tc.host || s.Token() != tc.tok {
				t.Errorf("got host %q token %q, want %q %q", s.Host(), s.Token(), tc.host, tc.tok)
			}
		})
	}
}

// Regression test for #42: storing or clearing the token saves only the
// config-file layer, whatever overrides are active.
func TestStoreTokenNeverPersistsOverrides(t *testing.T) {
	env := envOf(map[string]string{HostEnv: "https://env.example", TokenEnv: "env-token"})
	flags := Overrides{Host: "https://flag.example", Token: "flag-token"}
	cases := []struct {
		name  string
		token string
		want  string
	}{
		{"store", "new-token", "{\n  \"host\": \"https://file.example\",\n  \"token\": \"new-token\"\n}"},
		{"clear", "", "{\n  \"host\": \"https://file.example\"\n}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeConfig(t, `{"host":"https://file.example","token":"file-token"}`)
			s, err := Resolve(dir, env, flags)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.StoreToken(tc.token); err != nil {
				t.Fatal(err)
			}
			if got := readConfig(t, dir); got != tc.want {
				t.Errorf("saved %q, want %q", got, tc.want)
			}
			if s.Host() != "https://flag.example" || s.Token() != "flag-token" {
				t.Errorf("overrides should still win: %q %q", s.Host(), s.Token())
			}
		})
	}
}

func TestStoreTokenUpdatesTheEffectiveToken(t *testing.T) {
	dir := writeConfig(t, `{"host":"https://file.example","token":"old"}`)
	s, err := Resolve(dir, envOf(nil), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StoreToken("new"); err != nil {
		t.Fatal(err)
	}
	if s.Token() != "new" || s.Host() != "https://file.example" {
		t.Errorf("got %q %q", s.Host(), s.Token())
	}
}

func TestStoreTokenCreatesPrivateDirectoryAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := filepath.Join(t.TempDir(), "nested", "bookbeam")
	s, err := Resolve(dir, envOf(nil), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StoreToken("t"); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Errorf("directory permissions = %o, want 700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Errorf("file permissions = %o, want 600", got)
	}
	want := "{\n  \"host\": \"https://bookbeam.app\",\n  \"token\": \"t\"\n}"
	if got := readConfig(t, dir); got != want {
		t.Errorf("saved config = %q, want %q", got, want)
	}
}

func TestStoreTokenReportsDirectoryCreationFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Settings{path: filepath.Join(blocker, "sub", "config.json")}

	err := s.StoreToken("t")
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "mkdir" {
		t.Fatalf("expected mkdir path error, got %v", err)
	}
}

func TestStoreTokenReportsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	s, err := Resolve(dir, envOf(nil), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "config.json"), 0700); err != nil {
		t.Fatal(err)
	}
	var pathErr *fs.PathError
	if err := s.StoreToken("t"); !errors.As(err, &pathErr) || pathErr.Op != "open" {
		t.Fatalf("expected open path error, got %v", err)
	}
}

func TestResolveUnreadableConfigFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions not applicable on Windows")
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"token":"secret"}`), 0000); err != nil {
		t.Fatal(err)
	}

	s, err := Resolve(dir, envOf(nil), Overrides{})
	if err == nil || s != nil {
		t.Fatal("expected error loading unreadable config file")
	}
	want := "failed to read config file " + configPath + ": "
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("expected error to start with %q, got: %s", want, err)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("expected wrapped fs.ErrPermission, got: %v", err)
	}
}

func TestResolveInvalidConfigFile(t *testing.T) {
	for name, body := range map[string]string{"invalid JSON": `{not-valid-json`, "empty": ``} {
		t.Run(name, func(t *testing.T) {
			dir := writeConfig(t, body)
			configPath := filepath.Join(dir, "config.json")

			s, err := Resolve(dir, envOf(nil), Overrides{})
			if err == nil || s != nil {
				t.Fatal("expected error loading invalid config file")
			}
			want := "failed to parse config file " + configPath + ": "
			if !strings.HasPrefix(err.Error(), want) {
				t.Errorf("expected error to start with %q, got: %s", want, err)
			}
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Errorf("expected wrapped json.SyntaxError, got: %v", err)
			}
		})
	}
}
