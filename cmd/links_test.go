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

func TestLinksAndDownloadersCommands(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":              201,
					"book_project_id": 5,
					"slug":            "promo-lead-magnet",
					"title":           "Free Sample",
					"created_at":      "2026-09-13T10:00:00Z",
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/projects/5/downloaders" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         1,
						"email":      "reader@example.com",
						"link_name":  "Free Sample",
						"created_at": "2026-09-13T11:00:00Z",
					},
				},
				"current_page": 1,
				"last_page":    1,
				"total":        1,
			})
			return
		}

		if r.URL.Path == "/api/v1/projects/5/export-downloaders" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("Email,Link,Date\nreader@example.com,Free Sample,2026-09-13\n"))
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	err := linksListCmd.RunE(linksListCmd, []string{"5"})
	if err != nil {
		t.Fatalf("unexpected links list error: %v", err)
	}
	if !strings.Contains(buf.String(), "promo-lead-magnet") {
		t.Errorf("expected output to contain slug promo-lead-magnet, got %s", buf.String())
	}

	buf.Reset()
	err = downloadersListCmd.RunE(downloadersListCmd, []string{"5"})
	if err != nil {
		t.Fatalf("unexpected downloaders list error: %v", err)
	}
	if !strings.Contains(buf.String(), "reader@example.com") {
		t.Errorf("expected output to contain reader email, got %s", buf.String())
	}

	tempDir, err := os.MkdirTemp("", "bookbeam-export-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	exportFile := filepath.Join(tempDir, "export.csv")
	flagDownloaderOutput = exportFile

	buf.Reset()
	err = downloadersExportCmd.RunE(downloadersExportCmd, []string{"5"})
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	data, err := os.ReadFile(exportFile)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if !strings.Contains(string(data), "reader@example.com") {
		t.Errorf("expected CSV to contain reader@example.com, got %s", string(data))
	}
}
