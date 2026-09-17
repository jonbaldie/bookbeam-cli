package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoBareLoginCommandHint guards against user-facing text that tells
// people to run "bookbeam login", a command that does not exist; the real
// invocation is "bookbeam auth login". See issue #4.
func TestNoBareLoginCommandHint(t *testing.T) {
	bareLogin := regexp.MustCompile(`bookbeam login\b`)
	authLogin := regexp.MustCompile(`bookbeam auth login\b`)

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".md" {
			return nil
		}
		if strings.HasSuffix(path, "login_hint_test.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for line := range strings.SplitSeq(string(data), "\n") {
			if bareLogin.MatchString(line) && !authLogin.MatchString(line) {
				t.Errorf("%s: found bare 'bookbeam login' hint, want 'bookbeam auth login': %s", path, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
