package integration_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/config"
)

func TestDownloadersExportStdout(t *testing.T) {
	const csv = "Email\nx@example.com\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/12/export-downloaders" || r.Header.Get("Authorization") != "Bearer fixture-token" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, csv)
	}))
	defer server.Close()

	for _, tc := range []struct {
		name   string
		flags  []string
		stderr string
	}{
		{"normal", nil, "✓ Exported 20 bytes to stdout\n"},
		{"quiet", []string{"--quiet"}, ""},
		{"json", []string{"--json"}, "✓ Exported 20 bytes to stdout\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"downloaders", "export", "12", "-o", "-"}, tc.flags...)
			command := exec.Command(binary, args...)
			command.Dir = dir
			for _, e := range os.Environ() {
				if !strings.HasPrefix(e, "BOOKBEAM_") && !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "USERPROFILE=") {
					command.Env = append(command.Env, e)
				}
			}
			command.Env = append(command.Env, "BOOKBEAM_HOST="+server.URL, "BOOKBEAM_TOKEN=fixture-token", "HOME="+dir, "USERPROFILE="+dir, config.ConfigDirEnv+"="+filepath.Join(dir, "config"))
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				t.Fatalf("export failed: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			if stdout.String() != csv {
				t.Errorf("stdout = %q, want %q", stdout.String(), csv)
			}
			if stderr.String() != tc.stderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.stderr)
			}
			if _, err := os.Stat(filepath.Join(dir, "-")); !os.IsNotExist(err) {
				t.Errorf("file named dash exists or stat failed: %v", err)
			}
		})
	}
}
