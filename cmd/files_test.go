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
	a := &app{}
	filesListCmd := filesListCmd(a)
	filesUploadCmd := filesUploadCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/projects/10/files" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":              55,
					"book_project_id": 10,
					"filename":        "novel.epub",
					"file_type":       "epub",
					"file_size":       1024,
					"downloads_count": 12,
					"created_at":      "2026-09-13T10:00:00Z",
				},
			}})
			return
		}

		if r.URL.Path == "/api/v1/projects/10/files" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":        56,
				"filename":  "sample.epub",
				"file_type": "epub",
			}})
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

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

func TestResolveDownloadDestination(t *testing.T) {
	tests := []struct {
		name       string
		outputFlag string
		header     string
		projectID  string
		fileID     string
		expected   string
	}{
		{
			name:       "explicit output flag wins unconditionally",
			outputFlag: "custom/path.epub",
			header:     `attachment; filename="server.epub"`,
			projectID:  "1",
			fileID:     "2",
			expected:   "custom/path.epub",
		},
		{
			name:       "valid content-disposition header",
			outputFlag: "",
			header:     `attachment; filename="dogfood-sampler.epub"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "dogfood-sampler.epub",
		},
		{
			name:       "inline content-disposition with quotes",
			outputFlag: "",
			header:     `inline; filename="report.pdf"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "report.pdf",
		},
		{
			name:       "content-disposition without quotes",
			outputFlag: "",
			header:     `attachment; filename=book.mobi`,
			projectID:  "7",
			fileID:     "16",
			expected:   "book.mobi",
		},
		{
			name:       "path traversal with forward slashes is sanitized",
			outputFlag: "",
			header:     `attachment; filename="../../etc/shadow"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "shadow",
		},
		{
			name:       "path traversal with backward slashes is sanitized",
			outputFlag: "",
			header:     `attachment; filename="..\\..\\windows\\system32\\calc.exe"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "calc.exe",
		},
		{
			name:       "nested subdirectories stripped to base filename",
			outputFlag: "",
			header:     `attachment; filename="folder/subfolder/novel.epub"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "novel.epub",
		},
		{
			name:       "empty header falls back to default pattern",
			outputFlag: "",
			header:     "",
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "unparseable header falls back to default pattern",
			outputFlag: "",
			header:     "malformed ;;;; ===",
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "missing filename parameter falls back to default pattern",
			outputFlag: "",
			header:     "attachment",
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "filename consisting solely of dots falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename=".."`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "filename consisting of single dot falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename="."`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "filename consisting solely of slashes falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename="///"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "filename consisting solely of backslashes falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename="\\\\\\"`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "empty filename falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename=""`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
		{
			name:       "whitespace only filename falls back to default pattern",
			outputFlag: "",
			header:     `attachment; filename="   "`,
			projectID:  "7",
			fileID:     "16",
			expected:   "project-7-file-16",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDownloadDestination(tc.outputFlag, tc.header, tc.projectID, tc.fileID)
			if got != tc.expected {
				t.Errorf("resolveDownloadDestination(%q, %q, %q, %q) = %q, want %q",
					tc.outputFlag, tc.header, tc.projectID, tc.fileID, got, tc.expected)
			}
		})
	}
}

func TestExtractDispositionFilename(t *testing.T) {
	if got := extractDispositionFilename(`attachment; filename="test.epub"`); got != "test.epub" {
		t.Errorf("expected test.epub, got %q", got)
	}
	if got := extractDispositionFilename("invalid;;;;"); got != "" {
		t.Errorf("expected empty string for invalid header, got %q", got)
	}
	if got := extractDispositionFilename("attachment"); got != "" {
		t.Errorf("expected empty string for missing filename, got %q", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"   ", ""},
		{".", ""},
		{"..", ""},
		{"/", ""},
		{"\\", ""},
		{"///", ""},
		{"\\\\", ""},
		{"book.pdf", "book.pdf"},
		{"  book.pdf  ", "book.pdf"},
		{"dir/book.pdf", "book.pdf"},
		{"dir\\book.pdf", "book.pdf"},
		{"../../book.pdf", "book.pdf"},
		{"..\\..\\book.pdf", "book.pdf"},
		{"dir/  book.pdf  ", "book.pdf"},
		{"/   /", ""},
	}

	for _, c := range cases {
		got := sanitizeFilename(c.input)
		if got != c.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestFilesDownloadCmd(t *testing.T) {
	a := &app{}
	filesDownloadCmd := filesDownloadCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/projects/7/files/16/download" {
			w.Header().Set("Content-Disposition", `attachment; filename="novel.epub"`)
			w.Write([]byte("book content bytes"))
			return
		}
		if r.URL.Path == "/api/v1/projects/7/files/99/download" {
			// No Content-Disposition header
			w.Write([]byte("fallback book bytes"))
			return
		}
		if r.URL.Path == "/api/v1/projects/7/files/404/download" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "unexpected", http.StatusBadRequest)
	}))
	defer ts.Close()

	t.Run("download with Content-Disposition header", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)

		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDownloadCmd.Flags().Set("output", "")
		err := filesDownloadCmd.RunE(filesDownloadCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tempDir, "novel.epub"))
		if err != nil || string(data) != "book content bytes" {
			t.Fatalf("file not written properly: %v", err)
		}
		if !strings.Contains(buf.String(), "novel.epub") {
			t.Errorf("expected output to mention novel.epub, got: %s", buf.String())
		}
	})

	t.Run("download without Content-Disposition falls back to project-id-file-id", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)

		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDownloadCmd.Flags().Set("output", "")
		err := filesDownloadCmd.RunE(filesDownloadCmd, []string{"7", "99"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tempDir, "project-7-file-99"))
		if err != nil || string(data) != "fallback book bytes" {
			t.Fatalf("fallback file not written properly: %v", err)
		}
	})

	t.Run("download with explicit output flag", func(t *testing.T) {
		tempDir := t.TempDir()

		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		customDest := filepath.Join(tempDir, "custom.epub")
		filesDownloadCmd.Flags().Set("output", customDest)
		defer filesDownloadCmd.Flags().Set("output", "")

		err := filesDownloadCmd.RunE(filesDownloadCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(customDest)
		if err != nil || string(data) != "book content bytes" {
			t.Fatalf("custom file not written properly: %v", err)
		}
	})

	t.Run("download API error", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDownloadCmd.Flags().Set("output", "")
		err := filesDownloadCmd.RunE(filesDownloadCmd, []string{"7", "404"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("download invalid destination path error", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDownloadCmd.Flags().Set("output", "/non/existent/dir/file.epub")
		defer filesDownloadCmd.Flags().Set("output", "")

		err := filesDownloadCmd.RunE(filesDownloadCmd, []string{"7", "16"})
		if err == nil {
			t.Fatal("expected error for invalid destination path, got nil")
		}
	})
}

func TestFilesDeleteCmd(t *testing.T) {
	a := &app{}
	filesDeleteCmd := filesDeleteCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/projects/7/files/16" && r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"file deleted"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	t.Run("force delete success", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "true")
		defer filesDeleteCmd.Flags().Set("force", "false")

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "Deleted file #16") {
			t.Errorf("expected output to mention deleted file, got: %s", buf.String())
		}
	})

	t.Run("force delete JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: true}
		defer func() { a.printer.JSON = false }()
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "true")
		defer filesDeleteCmd.Flags().Set("force", "false")

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "file deleted") {
			t.Errorf("expected json output, got: %s", buf.String())
		}
	})

	t.Run("json output without force skips prompt", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: true}
		defer func() { a.printer.JSON = false }()
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "false")

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "file deleted") {
			t.Errorf("expected json output, got: %s", buf.String())
		}
	})

	t.Run("delete API error", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "true")
		defer filesDeleteCmd.Flags().Set("force", "false")

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "999"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("interactive prompt confirm y", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "false")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		w.Write([]byte("y\n"))
		w.Close()
		defer func() { os.Stdin = oldStdin }()

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "Deleted file #16") {
			t.Errorf("expected delete message, got: %s", buf.String())
		}
	})

	t.Run("interactive prompt cancel n", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		filesDeleteCmd.Flags().Set("force", "false")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		w.Write([]byte("n\n"))
		w.Close()
		defer func() { os.Stdin = oldStdin }()

		err := filesDeleteCmd.RunE(filesDeleteCmd, []string{"7", "16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "Cancelled.") {
			t.Errorf("expected cancelled message, got: %s", buf.String())
		}
	})
}

func TestFilesListAndUploadVariations(t *testing.T) {
	a := &app{}
	filesListCmd := filesListCmd(a)
	filesUploadCmd := filesUploadCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/projects/1/files" && r.Method == http.MethodGet {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		if r.URL.Path == "/api/v1/projects/2/files" && r.Method == http.MethodGet {
			w.Write([]byte(`{"data":[{"id":1,"book_project_id":2,"filename":"test.epub","file_type":"epub","file_size":100,"downloads_count":1,"created_at":"2026-09-01"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	t.Run("empty list prints info", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: false}
		a.apiCli = client.New(ts.URL, "test-token")

		err := filesListCmd.RunE(filesListCmd, []string{"1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "No files attached") {
			t.Errorf("expected no files message, got %s", buf.String())
		}
	})

	t.Run("list JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		a.printer = &output.Printer{Out: &buf, JSON: true}
		defer func() { a.printer.JSON = false }()
		a.apiCli = client.New(ts.URL, "test-token")

		err := filesListCmd.RunE(filesListCmd, []string{"2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "test.epub") {
			t.Errorf("expected json output to contain test.epub, got %s", buf.String())
		}
	})

	t.Run("upload invalid file extension", func(t *testing.T) {
		tempDir := t.TempDir()
		txtFile := filepath.Join(tempDir, "notes.txt")
		_ = os.WriteFile(txtFile, []byte("some notes"), 0644)

		a.apiCli = client.New(ts.URL, "test-token")
		err := filesUploadCmd.RunE(filesUploadCmd, []string{"1", txtFile})
		if err == nil || !strings.Contains(err.Error(), "unsupported file extension") {
			t.Fatalf("expected unsupported extension error, got: %v", err)
		}
	})

	t.Run("upload non-existent file", func(t *testing.T) {
		a.apiCli = client.New(ts.URL, "test-token")
		err := filesUploadCmd.RunE(filesUploadCmd, []string{"1", "/non/existent/file.epub"})
		if err == nil || !strings.Contains(err.Error(), "file not found") {
			t.Fatalf("expected file not found error, got: %v", err)
		}
	})
}
