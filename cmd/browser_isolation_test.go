package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserCommandIsIsolated(t *testing.T) {
	for _, url := range []string{"", "https://example.com/verify"} {
		t.Run(url, func(t *testing.T) {
			command := browserCommand(runtime.GOOS, url)
			dir := filepath.Dir(command.Path)
			if dir != filepath.SplitList(os.Getenv("PATH"))[0] || !strings.HasPrefix(filepath.Base(dir), "bookbeam-browser-stubs-") {
				t.Fatalf("browser command resolved outside test stubs: %q", command.Path)
			}
			// Windows resolves the .bat stub, but exec.Cmd cannot run batch files directly.
			if runtime.GOOS != "windows" {
				if err := command.Run(); err != nil {
					t.Fatalf("run isolated browser command: %v", err)
				}
			}
		})
	}
}
