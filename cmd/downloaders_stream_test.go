package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDownloaderExportStreamsAndReplacesOnCompletion(t *testing.T) {
	first := make(chan struct{})
	finish := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "first\n")
		w.(http.Flusher).Flush()
		close(first)
		<-finish
		fmt.Fprint(w, "second\n")
	}))
	defer s.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(dest, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	a, _ := ntdApp(s.URL, false)
	cmd := downloadersExportCmd(a)
	_ = cmd.Flags().Set("output", dest)
	done := make(chan error, 1)
	go func() { done <- runCommand(cmd, []string{"12"}) }()
	select {
	case <-first:
	case err := <-done:
		t.Fatalf("export ended before the first chunk: %v", err)
	case <-time.After(5 * time.Second):
		close(finish)
		t.Fatal("export did not start")
	}
	defer func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(dir, ".bookbeam-csv-*"))
		if len(files) == 1 {
			b, _ := os.ReadFile(files[0])
			if string(b) == "first\n" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("first chunk was not staged before response completed")
		}
		time.Sleep(time.Millisecond)
	}
	old, _ := os.ReadFile(dest)
	if string(old) != "old" {
		t.Fatalf("destination before completion = %q", old)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "first\nsecond\n" {
		t.Fatalf("destination = %q", got)
	}
	info, _ := os.Stat(dest)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".bookbeam-csv-*"))
	if len(files) != 0 {
		t.Fatalf("temporary files: %v", files)
	}
}

func TestDownloaderExportInterruptedResponsePreservesDestination(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "partial")
	}))
	defer s.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(dest, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	a, _ := ntdApp(s.URL, false)
	cmd := downloadersExportCmd(a)
	_ = cmd.Flags().Set("output", dest)
	if err := runCommand(cmd, []string{"12"}); err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("error = %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("destination = %q", got)
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".bookbeam-csv-*"))
	if len(files) != 0 {
		t.Fatalf("temporary files: %v", files)
	}
}
