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

	resetProjectFlags()
	defer resetProjectFlags()

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
	_ = projectsCreateCmd.Flags().Set("title", "New Novel")
	err = projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	if !strings.Contains(buf.String(), "Created book project #102: New Novel") {
		t.Errorf("expected created message, got %s", buf.String())
	}
}

func resetProjectFlags() {
	_ = projectsListCmd.Flags().Set("page", "1")
	_ = projectsCreateCmd.Flags().Set("title", "")
	_ = projectsCreateCmd.Flags().Set("description", "")
	_ = projectsCreateCmd.Flags().Set("cover", "")
	_ = projectsUpdateCmd.Flags().Set("title", "")
	_ = projectsUpdateCmd.Flags().Set("description", "")
	_ = projectsUpdateCmd.Flags().Set("cover", "")
	_ = projectsUpdateCmd.Flags().Set("remove-cover", "false")
	_ = projectsDeleteCmd.Flags().Set("force", "false")
	_ = projectsNewsletterCmd.Flags().Set("list-id", "")
	_ = projectsNewsletterCmd.Flags().Set("tags", "")
}

func TestProjectsUpdateConflictingCoverFlags(t *testing.T) {
	tempDir := t.TempDir()
	coverFile := filepath.Join(tempDir, "cover.jpg")
	if err := os.WriteFile(coverFile, []byte("fake-cover-content"), 0644); err != nil {
		t.Fatal(err)
	}

	requestReceived := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 42}})
	}))
	defer ts.Close()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	resetProjectFlags()
	defer resetProjectFlags()

	_ = projectsUpdateCmd.Flags().Set("cover", coverFile)
	_ = projectsUpdateCmd.Flags().Set("remove-cover", "true")

	err := projectsUpdateCmd.RunE(projectsUpdateCmd, []string{"42"})
	if err == nil {
		t.Fatal("expected error when passing both --cover and --remove-cover, but got nil")
	}
	if !strings.Contains(err.Error(), "cannot specify both --cover and --remove-cover") {
		t.Errorf("expected conflict message, got: %v", err)
	}

	if requestReceived {
		t.Errorf("expected no HTTP request to be sent when flags conflict, but request was received")
	}
}

func TestProjectsUpdateRemoveCoverJSON(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/projects/42" && r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 42}})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	resetProjectFlags()
	defer resetProjectFlags()

	_ = projectsUpdateCmd.Flags().Set("remove-cover", "true")

	err := projectsUpdateCmd.RunE(projectsUpdateCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val, ok := receivedBody["remove_cover"].(bool); !ok || !val {
		t.Errorf("expected remove_cover to be true, got %v", receivedBody["remove_cover"])
	}

	if !strings.Contains(buf.String(), "Updated book project #42") {
		t.Errorf("expected success message, got %s", buf.String())
	}
}

func TestProjectsUpdateMultipartFieldsWithRemoveCover(t *testing.T) {
	fields := buildProjectUpdateMultipartFields("My Title", "My Description", true)
	if fields["title"] != "My Title" {
		t.Errorf("expected title 'My Title', got %q", fields["title"])
	}
	if fields["description"] != "My Description" {
		t.Errorf("expected description 'My Description', got %q", fields["description"])
	}
	if fields["remove_cover"] != "true" {
		t.Errorf("expected remove_cover 'true', got %q", fields["remove_cover"])
	}
}

func TestProjectsUpdateMultipartRemoveCoverServer(t *testing.T) {
	tempDir := t.TempDir()
	dummyFile := filepath.Join(tempDir, "cover.jpg")
	if err := os.WriteFile(dummyFile, []byte("fake-cover"), 0644); err != nil {
		t.Fatal(err)
	}

	var removeCoverVal string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		removeCoverVal = r.FormValue("remove_cover")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 42}})
	}))
	defer ts.Close()

	c := client.New(ts.URL, "test-token")
	fields := buildProjectUpdateMultipartFields("Sample", "", true)
	_, err := c.PostMultipart("/api/v1/projects/42", fields, "cover_image", dummyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removeCoverVal != "true" {
		t.Errorf("expected server to receive remove_cover=true in multipart body, got %q", removeCoverVal)
	}
}

func TestProjectsListVariations(t *testing.T) {
	var requestedPage string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPage = r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if requestedPage == "99" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":         []any{},
				"current_page": 99,
				"last_page":    1,
				"total":        0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": 1, "title": "P1", "files_count": 0, "signup_links_count": 0, "created_at": "2026-09-01"},
			},
			"current_page": 1,
			"last_page":    1,
			"total":        1,
		})
	}))
	defer ts.Close()

	resetProjectFlags()
	defer resetProjectFlags()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	// Test page flag
	_ = projectsListCmd.Flags().Set("page", "2")
	err := projectsListCmd.RunE(projectsListCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestedPage != "2" {
		t.Errorf("expected page 2, got %s", requestedPage)
	}

	// Test empty list message
	buf.Reset()
	_ = projectsListCmd.Flags().Set("page", "99")
	err = projectsListCmd.RunE(projectsListCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No book projects found") {
		t.Errorf("expected empty message, got %s", buf.String())
	}

	// Test JSON output
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	_ = projectsListCmd.Flags().Set("page", "1")
	err = projectsListCmd.RunE(projectsListCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"title": "P1"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}

func TestProjectsGetVariations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/42" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                 42,
					"title":              "Galaxy Hitchhiker",
					"description":        "Don't panic",
					"cover_image_url":    "https://example.com/cover.jpg",
					"files_count":        2,
					"signup_links_count": 1,
					"created_at":         "2026-09-01",
				},
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

	err := projectsGetCmd.RunE(projectsGetCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Galaxy Hitchhiker") {
		t.Errorf("expected project title in output, got %s", buf.String())
	}

	// Test JSON output
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	err = projectsGetCmd.RunE(projectsGetCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"Galaxy Hitchhiker"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}

	// Test API error
	err = projectsGetCmd.RunE(projectsGetCmd, []string{"999"})
	if err == nil {
		t.Fatal("expected error for non-existent project, got nil")
	}
}

func TestProjectsCreateVariations(t *testing.T) {
	var receivedPost map[string]string
	var receivedMultipartFields map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			_ = r.ParseMultipartForm(10 << 20)
			receivedMultipartFields = make(map[string]string)
			for k, v := range r.MultipartForm.Value {
				if len(v) > 0 {
					receivedMultipartFields[k] = v[0]
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 103, "title": receivedMultipartFields["title"]}})
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedPost)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 103, "title": receivedPost["title"]}})
	}))
	defer ts.Close()

	resetProjectFlags()
	defer resetProjectFlags()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	// Missing title error
	err := projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "--title is required") {
		t.Fatalf("expected --title is required error, got: %v", err)
	}

	// Create with title and description
	_ = projectsCreateCmd.Flags().Set("title", "Desc Book")
	_ = projectsCreateCmd.Flags().Set("description", "A great book")
	err = projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPost["title"] != "Desc Book" || receivedPost["description"] != "A great book" {
		t.Errorf("unexpected post body: %v", receivedPost)
	}

	// Create with cover (multipart)
	dummyCover := filepath.Join(t.TempDir(), "cover.png")
	_ = os.WriteFile(dummyCover, []byte("png"), 0644)
	_ = projectsCreateCmd.Flags().Set("title", "Cover Book")
	_ = projectsCreateCmd.Flags().Set("description", "With cover")
	_ = projectsCreateCmd.Flags().Set("cover", dummyCover)
	err = projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMultipartFields["title"] != "Cover Book" || receivedMultipartFields["description"] != "With cover" {
		t.Errorf("unexpected multipart fields: %v", receivedMultipartFields)
	}

	// Create with JSON output
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	_ = projectsCreateCmd.Flags().Set("cover", "")
	_ = projectsCreateCmd.Flags().Set("title", "JSON Book")
	err = projectsCreateCmd.RunE(projectsCreateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"JSON Book"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}

func TestProjectsUpdateVariations(t *testing.T) {
	var receivedPut map[string]any
	var receivedMultipartFields map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			_ = r.ParseMultipartForm(10 << 20)
			receivedMultipartFields = make(map[string]string)
			for k, v := range r.MultipartForm.Value {
				if len(v) > 0 {
					receivedMultipartFields[k] = v[0]
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 42}})
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedPut)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 42}})
	}))
	defer ts.Close()

	resetProjectFlags()
	defer resetProjectFlags()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	// Update title and description
	_ = projectsUpdateCmd.Flags().Set("title", "Updated Title")
	_ = projectsUpdateCmd.Flags().Set("description", "Updated Desc")
	err := projectsUpdateCmd.RunE(projectsUpdateCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPut["title"] != "Updated Title" || receivedPut["description"] != "Updated Desc" {
		t.Errorf("unexpected put payload: %v", receivedPut)
	}

	// Update cover
	dummyCover := filepath.Join(t.TempDir(), "new-cover.jpg")
	_ = os.WriteFile(dummyCover, []byte("new-cover"), 0644)
	_ = projectsUpdateCmd.Flags().Set("cover", dummyCover)
	_ = projectsUpdateCmd.Flags().Set("title", "Cover Update")
	_ = projectsUpdateCmd.Flags().Set("description", "Cover Desc")
	err = projectsUpdateCmd.RunE(projectsUpdateCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMultipartFields["title"] != "Cover Update" || receivedMultipartFields["description"] != "Cover Desc" {
		t.Errorf("unexpected multipart fields: %v", receivedMultipartFields)
	}

	// Update JSON output
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	_ = projectsUpdateCmd.Flags().Set("cover", "")
	err = projectsUpdateCmd.RunE(projectsUpdateCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"id": 42`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}

func TestProjectsDeleteVariations(t *testing.T) {
	deleted := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "deleted"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	resetProjectFlags()
	defer resetProjectFlags()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	// Delete with force
	_ = projectsDeleteCmd.Flags().Set("force", "true")
	err := projectsDeleteCmd.RunE(projectsDeleteCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("expected delete request")
	}
	if !strings.Contains(buf.String(), "Deleted book project #42") {
		t.Errorf("expected deleted message, got %s", buf.String())
	}

	// Delete with JSON
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	_ = projectsDeleteCmd.Flags().Set("force", "false")
	err = projectsDeleteCmd.RunE(projectsDeleteCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"deleted"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}

func TestProjectsNewsletterVariations(t *testing.T) {
	var receivedPut map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&receivedPut)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "success"})
	}))
	defer ts.Close()

	resetProjectFlags()
	defer resetProjectFlags()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	apiCli = client.New(ts.URL, "test-token")

	// Missing list-id error
	err := projectsNewsletterCmd.RunE(projectsNewsletterCmd, []string{"42"})
	if err == nil || !strings.Contains(err.Error(), "--list-id is required") {
		t.Fatalf("expected --list-id is required error, got: %v", err)
	}

	// With list-id and tags
	_ = projectsNewsletterCmd.Flags().Set("list-id", "list-abc")
	_ = projectsNewsletterCmd.Flags().Set("tags", "vip,beta")
	err = projectsNewsletterCmd.RunE(projectsNewsletterCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPut["newsletter_list_id"] != "list-abc" || receivedPut["newsletter_tags"] != "vip,beta" {
		t.Errorf("unexpected newsletter payload: %v", receivedPut)
	}

	// With JSON output
	buf.Reset()
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: true}
	err = projectsNewsletterCmd.RunE(projectsNewsletterCmd, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"message": "success"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}



