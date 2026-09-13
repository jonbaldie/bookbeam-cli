package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestProjectsListAndCreate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":                 101,
						"title":              "The Quantum Paradox",
						"files_count":        3,
						"signup_links_count": 2,
						"created_at":         "2026-09-13T12:00:00Z",
					},
				},
				"current_page": 1,
				"last_page":    1,
				"total":        1,
			})
			return
		}

		if r.URL.Path == "/api/v1/projects" && r.Method == http.MethodPost {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":    102,
				"title": body["title"],
			}})
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	err := projectsListCmd.RunE(projectsListCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}

	if !strings.Contains(buf.String(), "The Quantum Paradox") {
		t.Errorf("expected list to show project title, got %s", buf.String())
	}

	buf.Reset()
	flagProjectTitle = "New Novel"
	err = projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	if !strings.Contains(buf.String(), "Created book project #102: New Novel") {
		t.Errorf("expected created message, got %s", buf.String())
	}
}
