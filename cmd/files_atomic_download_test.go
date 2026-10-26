package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestFilesDownloadFailurePreservesDestination(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			name := "header"
			if explicit {
				name = "output"
			}
			if existing {
				name += "/existing"
			} else {
				name += "/absent"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				dest := filepath.Join(dir, "keep.epub")
				const original = "complete original book"
				if existing {
					if err := os.WriteFile(dest, []byte(original), 0600); err != nil {
						t.Fatal(err)
					}
				}
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Disposition", `attachment; filename="keep.epub"`)
					w.Header().Set("Content-Length", "1000")
					_, _ = w.Write([]byte("partial book"))
				}))
				defer ts.Close()
				var out bytes.Buffer
				a := &app{apiCli: client.New(ts.URL, "test-token"), printer: &output.Printer{Out: &out}}
				cmd := filesDownloadCmd(a)
				if explicit {
					if err := cmd.Flags().Set("output", dest); err != nil {
						t.Fatal(err)
					}
				}
				err := runCommand(cmd, []string{"7", "16"})
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("expected interrupted body, got %v", err)
				}
				data, err := os.ReadFile(dest)
				if existing {
					if err != nil || string(data) != original {
						t.Errorf("existing destination changed: data=%q err=%v", data, err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("failed download left final destination: data=%q err=%v", data, err)
				}
				if strings.Contains(out.String(), "Download complete") {
					t.Errorf("failure reported success: %s", out.String())
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Name() != "keep.epub" {
						t.Errorf("temporary download was not cleaned up: %s", entry.Name())
					}
				}
			})
		}
	}
}
