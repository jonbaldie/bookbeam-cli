package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Regression test for #77: a host without a scheme fails once, at load,
// naming --host, before any command runs or stores a token.
func TestSchemelessHostFailsAtLoad(t *testing.T) {
	const want = `failed to load configuration: invalid --host "127.0.0.1:18787": missing http:// or https://`
	for _, args := range [][]string{
		{"links", "list", "7"},
		{"auth", "login", "--token", "t"},
	} {
		t.Run(args[0], func(t *testing.T) {
			_, err := ntdRoot(t, append([]string{"--host", "127.0.0.1:18787"}, args...)...)
			if err == nil || err.Error() != want {
				t.Fatalf("want error %q, got %v", want, err)
			}
			_, statErr := os.Stat(filepath.Join(os.Getenv("BOOKBEAM_CONFIG_DIR"), "config.json"))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("config.json should not be written: %v", statErr)
			}
		})
	}
}
