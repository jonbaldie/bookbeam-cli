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
	a := &app{}
	downloadersExportCmd := downloadersExportCmd(a)
	downloadersListCmd := downloadersListCmd(a)
	linksListCmd := linksListCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":              201,
					"book_project_id": 5,
					"slug":            "promo-lead-magnet",
					"title":           "Free Sample",
					"created_at":      "2026-09-13T10:00:00Z",
				},
			}})
			return
		}

		if r.URL.Path == "/api/v1/projects/5/downloaders" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":                1,
						"email":             "reader@example.com",
						"signup_link_title": "Free Sample",
						"signed_up_at":      "2026-09-13T11:00:00Z",
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
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

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
	_ = downloadersExportCmd.Flags().Set("output", exportFile)

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

func setupLinksMockServer(t *testing.T, putBodyCapture *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":              201,
					"book_project_id": 5,
					"slug":            "promo-lead-magnet",
					"title":           "Reader Magnet",
					"opt_in_text":     "Join my VIP mailing list",
					"created_at":      "2026-09-13T10:00:00Z",
				},
			}})
			return
		}

		if r.URL.Path == "/api/v1/projects/5/links/201" && r.Method == http.MethodPut {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if putBodyCapture != nil {
				*putBodyCapture = body
			}
			if _, ok := body["title"]; !ok {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"message": "The title field is required.",
					"errors":  map[string]any{"title": []string{"The title field is required."}},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          201,
					"title":       body["title"],
					"opt_in_text": body["opt_in_text"],
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
}

func TestLinksUpdateOnlyConsent(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := setupLinksMockServer(t, &receivedPutBody)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("consent", "Updated consent text")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("links update failed with error: %v", err)
	}

	if receivedPutBody["title"] != "Reader Magnet" {
		t.Errorf("expected PUT title to preserve existing 'Reader Magnet', got %v", receivedPutBody["title"])
	}
	if receivedPutBody["opt_in_text"] != "Updated consent text" {
		t.Errorf("expected PUT opt_in_text to be 'Updated consent text', got %v", receivedPutBody["opt_in_text"])
	}
}

func TestLinksUpdateOnlyTitlePreservesConsent(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := setupLinksMockServer(t, &receivedPutBody)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("title", "New Magnet Title")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("links update failed with error: %v", err)
	}

	if receivedPutBody["title"] != "New Magnet Title" {
		t.Errorf("expected PUT title to be 'New Magnet Title', got %v", receivedPutBody["title"])
	}
	if receivedPutBody["opt_in_text"] != "Join my VIP mailing list" {
		t.Errorf("expected PUT opt_in_text to preserve existing 'Join my VIP mailing list', got %v", receivedPutBody["opt_in_text"])
	}
}

func TestLinksUpdateClearConsent(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := setupLinksMockServer(t, &receivedPutBody)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("clear-consent", "true")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("links update failed: %v", err)
	}

	if receivedPutBody["title"] != "Reader Magnet" {
		t.Errorf("expected PUT title to preserve 'Reader Magnet', got %v", receivedPutBody["title"])
	}
	// The API keeps fields a PUT omits, so clearing consent must send an explicit null.
	consent, ok := receivedPutBody["opt_in_text"]
	if !ok || consent != nil {
		t.Errorf("expected opt_in_text to be sent as explicit null when clearing consent, got present=%v value=%v", ok, consent)
	}
}

func TestLinksUpdateBothConsentAndClearConsentError(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)

	_ = linksUpdateCmd.Flags().Set("consent", "Some consent")
	_ = linksUpdateCmd.Flags().Set("clear-consent", "true")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err == nil {
		t.Fatal("expected error when both --consent and --clear-consent are specified, got nil")
	}
	if !strings.Contains(err.Error(), "cannot specify both --consent and --clear-consent") {
		t.Errorf("expected mutually exclusive error message, got: %v", err)
	}
}

func TestLinksUpdateLinkNotFound(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	ts := setupLinksMockServer(t, nil)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("consent", "New consent")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "999"})
	if err == nil {
		t.Fatal("expected error when link not found, got nil")
	}
	if !strings.Contains(err.Error(), "signup link #999 not found in project 5") {
		t.Errorf("expected not found error message, got: %v", err)
	}
}

func TestLinksUpdateBothTitleAndConsentExplicit(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/5/links/201" && r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&receivedPutBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": 201, "title": receivedPutBody["title"]},
			})
			return
		}
		// If GET /links is called, fail the test
		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodGet {
			t.Errorf("unexpected GET /links when both title and consent are explicitly provided")
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("title", "Explicit Title")
	_ = linksUpdateCmd.Flags().Set("consent", "Explicit Consent")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("links update failed: %v", err)
	}

	if receivedPutBody["title"] != "Explicit Title" {
		t.Errorf("expected 'Explicit Title', got %v", receivedPutBody["title"])
	}
	if receivedPutBody["opt_in_text"] != "Explicit Consent" {
		t.Errorf("expected 'Explicit Consent', got %v", receivedPutBody["opt_in_text"])
	}
}

func TestLinksUpdateNoFlagsPreservesAll(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := setupLinksMockServer(t, &receivedPutBody)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("links update with no flags failed: %v", err)
	}

	if receivedPutBody["title"] != "Reader Magnet" {
		t.Errorf("expected PUT title to preserve 'Reader Magnet', got %v", receivedPutBody["title"])
	}
	if receivedPutBody["opt_in_text"] != "Join my VIP mailing list" {
		t.Errorf("expected PUT opt_in_text to preserve 'Join my VIP mailing list', got %v", receivedPutBody["opt_in_text"])
	}
}

func TestLinksCreateAndForceDelete(t *testing.T) {
	a := &app{}
	linksCreateCmd := linksCreateCmd(a)
	linksDeleteCmd := linksDeleteCmd(a)
	linksUpdateCmd := linksUpdateCmd(a)
	var createdTitle string
	var createdConsent string
	var deletedLinkID string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodPost {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			createdTitle = body["title"]
			createdConsent = body["opt_in_text"]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         301,
					"title":      createdTitle,
					"slug":       "new-slug",
					"created_at": "2026-09-17T10:00:00Z",
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/projects/5/links/301" && r.Method == http.MethodDelete {
			deletedLinkID = "301"
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Deleted"})
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	// Verify flag defaults
	if f := linksUpdateCmd.Flags().Lookup("clear-consent"); f == nil || f.DefValue != "false" {
		t.Errorf("expected clear-consent flag default false, got %v", f)
	}
	if f := linksDeleteCmd.Flags().Lookup("force"); f == nil || f.DefValue != "false" {
		t.Errorf("expected force flag default false, got %v", f)
	}

	_ = linksCreateCmd.Flags().Set("title", "Fresh Link")
	_ = linksCreateCmd.Flags().Set("consent", "Consent Text")

	err := linksCreateCmd.RunE(linksCreateCmd, []string{"5"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if createdTitle != "Fresh Link" {
		t.Errorf("expected created title 'Fresh Link', got %q", createdTitle)
	}
	if createdConsent != "Consent Text" {
		t.Errorf("expected created consent 'Consent Text', got %q", createdConsent)
	}

	_ = linksDeleteCmd.Flags().Set("force", "true")
	err = linksDeleteCmd.RunE(linksDeleteCmd, []string{"5", "301"})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if deletedLinkID != "301" {
		t.Errorf("expected deletedLinkID '301', got %q", deletedLinkID)
	}
}

func TestLinksUpdateJSONOutput(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	ts := setupLinksMockServer(t, nil)
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: true}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	defer func() {
		a.printer = &output.Printer{Out: os.Stdout, JSON: false}
	}()

	_ = linksUpdateCmd.Flags().Set("title", "JSON Title")
	_ = linksUpdateCmd.Flags().Set("consent", "JSON Consent")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("update with JSON output failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"title": "JSON Title"`) {
		t.Errorf("expected JSON output containing title, got: %s", buf.String())
	}
}

func TestLinksUpdateEmptyExistingConsent(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	var receivedPutBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":              201,
					"book_project_id": 5,
					"slug":            "lead-magnet",
					"title":           "Existing Title",
					"opt_in_text":     "",
				},
			}})
			return
		}
		if r.URL.Path == "/api/v1/projects/5/links/201" && r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&receivedPutBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 201}})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_ = linksUpdateCmd.Flags().Set("title", "Updated Title Only")

	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if _, ok := receivedPutBody["opt_in_text"]; ok {
		t.Errorf("expected opt_in_text to not be set when existing is empty, got %v", receivedPutBody["opt_in_text"])
	}
}

func TestLinksUpdateAPIErrors(t *testing.T) {
	a := &app{}
	linksUpdateCmd := linksUpdateCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Internal Server Error 500"}`, http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	// Should fail fetching existing
	err := linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error containing 500 during fetch, got: %v", err)
	}

	// Should fail on PUT when title and consent are given
	_ = linksUpdateCmd.Flags().Set("title", "T")
	_ = linksUpdateCmd.Flags().Set("consent", "C")
	err = linksUpdateCmd.RunE(linksUpdateCmd, []string{"5", "201"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error containing 500 during PUT, got: %v", err)
	}
}

func TestFetchExistingLinkInvalidJSON(t *testing.T) {
	a := &app{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer ts.Close()

	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	_, err := fetchExistingLink(a, "5", "201")
	if err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected json unmarshal error with 'invalid character', got: %v", err)
	}
}

func TestLinksCreateVariations(t *testing.T) {
	a := &app{}
	linksCreateCmd := linksCreateCmd(a)
	var receivedPost map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/5/links" && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&receivedPost)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": 401, "title": receivedPost["title"], "slug": "slug-401"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: true}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	defer func() {
		a.printer = &output.Printer{Out: os.Stdout, JSON: false}
	}()

	_ = linksCreateCmd.Flags().Set("title", "Only Title")
	err := linksCreateCmd.RunE(linksCreateCmd, []string{"5"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, ok := receivedPost["opt_in_text"]; ok {
		t.Errorf("opt_in_text should not be set in POST payload when omitted, got %v", receivedPost["opt_in_text"])
	}
	if !strings.Contains(buf.String(), `"slug-401"`) {
		t.Errorf("expected JSON output containing slug-401, got: %s", buf.String())
	}
}

func TestLinksDeleteJSONOutput(t *testing.T) {
	a := &app{}
	linksDeleteCmd := linksDeleteCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/5/links/501" && r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Deleted successfully"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: true}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	defer func() {
		a.printer = &output.Printer{Out: os.Stdout, JSON: false}
	}()

	err := linksDeleteCmd.RunE(linksDeleteCmd, []string{"5", "501"})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"Deleted successfully"`) {
		t.Errorf("expected JSON output containing message, got: %s", buf.String())
	}
}
