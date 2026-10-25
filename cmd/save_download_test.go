package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveDownloadPublishesCompleteFile(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new"
		if existing {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "book.epub")
			if existing {
				if err := os.WriteFile(dest, []byte("old book"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			const content = "complete new book"
			n, err := saveDownload(dest, strings.NewReader(content))
			if err != nil || n != int64(len(content)) {
				t.Fatalf("saveDownload = (%d, %v)", n, err)
			}
			data, err := os.ReadFile(dest)
			if err != nil || string(data) != content {
				t.Fatalf("destination = %q, %v", data, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != "book.epub" {
				t.Fatalf("unexpected directory contents: %v, %v", entries, err)
			}
		})
	}
}

func TestSaveDownloadRenameFailureCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "book.epub")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	// A nonempty directory cannot be replaced by a downloaded file.
	marker := filepath.Join(dest, "keep")
	if err := os.WriteFile(marker, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := saveDownload(dest, strings.NewReader("new book"))
	if err == nil || !strings.HasPrefix(err.Error(), "failed to save downloaded file: ") || n != 0 {
		t.Fatalf("expected rename failure, got (%d, %v)", n, err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "original" {
		t.Fatalf("destination changed: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "book.epub" {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
}
