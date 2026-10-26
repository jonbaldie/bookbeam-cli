package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Real-root tests must never launch desktop applications, including when a
// mutant redirects control flow into a browser-opening path.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bookbeam-browser-stubs-")
	if err != nil {
		panic(err)
	}
	for name, script := range map[string]string{
		"open":         "#!/bin/sh\nexit 0\n",
		"xdg-open":     "#!/bin/sh\nexit 0\n",
		"rundll32.bat": "@exit /b 0\r\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			os.RemoveAll(dir)
			panic(err)
		}
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Errorf("isolate browser commands: %w", err))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
