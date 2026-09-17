package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bookbeam-integration-")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "bookbeam")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "..")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintln(os.Stderr, string(out), err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func runCLI(t *testing.T, server *httptest.Server, input string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "BOOKBEAM_HOST="+server.URL, "BOOKBEAM_TOKEN=fixture-token")
	command.Stdin = strings.NewReader(input)
	out, err := command.CombinedOutput()
	return string(out), err
}

func TestProjectDeletionRequiresAffirmativeConfirmation(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted.Store(true)
		}
		fmt.Fprint(w, `{"message":"Deleted"}`)
	}))
	defer server.Close()
	out, err := runCLI(t, server, "", "projects", "delete", "42")
	if err != nil || deleted.Load() || !strings.Contains(out, "Cancelled") {
		t.Fatalf("EOF must cancel without deleting: deleted=%v err=%v output=%s", deleted.Load(), err, out)
	}
}

func TestCreatedProjectDisplaysSavedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":42,"title":"Moon Book","description":"A story"}}`)
	}))
	defer server.Close()
	out, err := runCLI(t, server, "", "projects", "create", "--title", "Moon Book")
	if err != nil || !strings.Contains(out, "#42: Moon Book") {
		t.Fatalf("saved identity missing: %v %s", err, out)
	}
}

func TestResourceDetailsDisplayAPIValues(t *testing.T) {
	file := filepath.Join(t.TempDir(), "moon.pdf")
	if err := os.WriteFile(file, []byte("%PDF-1.4\nfixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, response string
		args, want     []string
	}{
		{"project", `{"data":{"id":42,"title":"Moon Book","description":"A story","files_count":2,"signup_links_count":3}}`, []string{"projects", "get", "42"}, []string{"42", "Moon Book", "A story"}},
		{"upload", `{"data":{"id":7,"filename":"moon.pdf","file_type":"pdf","file_size":16}}`, []string{"files", "upload", "42", file}, []string{"#7", "moon.pdf"}},
		{"link", `{"data":{"id":8,"title":"Launch","slug":"moon-launch"}}`, []string{"links", "create", "42", "--title", "Launch"}, []string{"#8", "/download/moon-launch"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, c.response) }))
			defer server.Close()
			out, err := runCLI(t, server, "", c.args...)
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in %s", want, out)
				}
			}
		})
	}
}

func TestListFilesDisplaysAttachedBook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":7,"filename":"moon.pdf","file_type":"pdf","file_size":329,"downloads_count":12}]}`)
	}))
	defer server.Close()
	out, err := runCLI(t, server, "", "files", "list", "42")
	if err != nil {
		t.Fatalf("listing failed: %v %s", err, out)
	}
	for _, want := range []string{"7", "moon.pdf", "PDF", "12"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s: %s", want, out)
		}
	}
}

func TestListLinksDisplaysShareableLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":8,"title":"Launch","slug":"moon-launch","signups_count":12}]}`)
	}))
	defer server.Close()
	out, err := runCLI(t, server, "", "links", "list", "42")
	if err != nil {
		t.Fatalf("listing failed: %v %s", err, out)
	}
	for _, want := range []string{"8", "Launch", "/download/moon-launch"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s: %s", want, out)
		}
	}
}

func TestDownloadSavesBookBytes(t *testing.T) {
	const book = "%PDF-1.4\nbook content\x00\xff"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			http.Error(w, "Unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, book)
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "book.pdf")
	out, err := runCLI(t, server, "", "files", "download", "42", "7", "-o", dest)
	if err != nil {
		t.Fatalf("download failed: %v %s", err, out)
	}
	saved, err := os.ReadFile(dest)
	if err != nil || string(saved) != book {
		t.Fatalf("download differs: %v %q", err, saved)
	}
}

func TestDownloaderTableDisplaysAttribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"reader_id":9,"email":"reader@example.com","signup_link_title":"Moon Launch","signed_up_at":"2026-09-13 22:47:34"}],"current_page":1,"last_page":1,"total":1}`)
	}))
	defer server.Close()
	out, err := runCLI(t, server, "", "downloaders", "list", "42")
	if err != nil {
		t.Fatalf("listing failed: %v %s", err, out)
	}
	for _, want := range []string{"reader@example.com", "Moon Launch", "2026-09-13 22:47:34"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s: %s", want, out)
		}
	}
}

func TestLinksUpdatePreservesTitleAndConsent(t *testing.T) {
	var lastPutBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/42/links" && r.Method == "GET" {
			fmt.Fprint(w, `{"data":[{"id":8,"title":"Original Title","opt_in_text":"Keep This Consent"}]}`)
			return
		}
		if r.URL.Path == "/api/v1/projects/42/links/8" && r.Method == "PUT" {
			_ = json.NewDecoder(r.Body).Decode(&lastPutBody)
			if _, ok := lastPutBody["title"]; !ok {
				w.WriteHeader(http.StatusUnprocessableEntity)
				fmt.Fprint(w, `{"message":"The title field is required."}`)
				return
			}
			fmt.Fprint(w, `{"data":{"id":8}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Update only consent
	out, err := runCLI(t, server, "", "links", "update", "42", "8", "--consent", "New Consent Text")
	if err != nil {
		t.Fatalf("links update failed: %v %s", err, out)
	}
	if !strings.Contains(out, "Updated signup link #8") {
		t.Errorf("expected success output, got: %s", out)
	}
	if lastPutBody["title"] != "Original Title" {
		t.Errorf("expected preserved title 'Original Title', got %v", lastPutBody["title"])
	}
	if lastPutBody["opt_in_text"] != "New Consent Text" {
		t.Errorf("expected opt_in_text 'New Consent Text', got %v", lastPutBody["opt_in_text"])
	}

	// Update only title
	out, err = runCLI(t, server, "", "links", "update", "42", "8", "--title", "Updated Title")
	if err != nil {
		t.Fatalf("links update failed: %v %s", err, out)
	}
	if lastPutBody["title"] != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %v", lastPutBody["title"])
	}
	if lastPutBody["opt_in_text"] != "Keep This Consent" {
		t.Errorf("expected preserved consent 'Keep This Consent', got %v", lastPutBody["opt_in_text"])
	}
}

func runCLIWithHome(t *testing.T, homeDir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(binary, args...)
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "BOOKBEAM_") && !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "USERPROFILE=") {
			env = append(env, e)
		}
	}
	command.Env = append(env, "HOME="+homeDir, "USERPROFILE="+homeDir)
	out, err := command.CombinedOutput()
	return string(out), err
}

func TestUnreadableConfigReportsPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions not applicable on Windows")
	}

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "bookbeam")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"token":"secret-token"}`), 0000); err != nil {
		t.Fatal(err)
	}

	out, err := runCLIWithHome(t, homeDir, "whoami")
	if err == nil {
		t.Fatalf("expected command to fail, got success. output: %s", out)
	}
	if strings.Contains(out, "Not authenticated") {
		t.Errorf("expected error not to report 'Not authenticated', got: %s", out)
	}
	if !strings.Contains(out, "config.json") {
		t.Errorf("expected error to name config path, got: %s", out)
	}
	if !strings.Contains(out, "permission denied") {
		t.Errorf("expected error to indicate permission denied, got: %s", out)
	}
}


