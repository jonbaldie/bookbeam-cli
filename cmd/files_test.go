package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestFilesListAndUpload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/10/files" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":              55,
					"book_project_id": 10,
					"filename":        "novel.epub",
					"format":          "epub",
					"file_size":       1024,
					"download_count":  12,
					"created_at":      "2026-09-13T10:00:00Z",
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/projects/10/files" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       56,
				"filename": "sample.epub",
				"format":   "epub",
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	err := filesListCmd.RunE(filesListCmd, []string{"10"})
	if err != nil {
		t.Fatalf("unexpected files list error: %v", err)
	}

	if !strings.Contains(buf.String(), "novel.epub") {
		t.Errorf("expected list output to contain novel.epub, got %s", buf.String())
	}

	tempDir, err := os.MkdirTemp("", "bookbeam-file-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dummyFile := filepath.Join(tempDir, "sample.epub")
	if err := os.WriteFile(dummyFile, []byte("dummy epub content"), 0644); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	err = filesUploadCmd.RunE(filesUploadCmd, []string{"10", dummyFile})
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	if !strings.Contains(buf.String(), "Uploaded file #56") {
		t.Errorf("expected upload success message, got %s", buf.String())
	}
}
