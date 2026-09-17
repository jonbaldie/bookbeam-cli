package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlowUploadSucceeds(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(31 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":7,"filename":"book.epub","file_type":"epub","file_size":1024}}`)
	}))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(file, make([]byte, 1024), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, server, "", "files", "upload", "42", file)
	if err != nil {
		t.Fatalf("upload failed: err=%v out=%s", err, out)
	}
}

func TestSlowDownloadSucceeds(t *testing.T) {
	t.Parallel()
	const bookContent = "%PDF-1.4\nslow download test content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(31 * time.Second)
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, bookContent)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "downloaded.pdf")
	out, err := runCLI(t, server, "", "files", "download", "42", "7", "-o", dest)
	if err != nil {
		t.Fatalf("download failed: err=%v out=%s", err, out)
	}

	saved, err := os.ReadFile(dest)
	if err != nil || string(saved) != bookContent {
		t.Fatalf("downloaded content mismatch: err=%v content=%q", err, saved)
	}
}

func TestShortAPICallTimesOut(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(32 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	out, err := runCLI(t, server, "", "projects", "list")
	if err == nil {
		t.Fatalf("expected error due to short API call timeout, got nil with output: %s", out)
	}
	if !strings.Contains(out, "context deadline exceeded") && !strings.Contains(out, "Client.Timeout exceeded") {
		t.Fatalf("expected deadline/timeout error message, got: %s", out)
	}
}
