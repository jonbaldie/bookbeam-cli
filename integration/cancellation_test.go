package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestSignalsCancelRemoteWork(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process signals")
	}
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		for _, operation := range []string{"short", "upload", "download"} {
			t.Run(signal.String()+"/"+operation, func(t *testing.T) {
				started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Read uploads before waiting for the response, so the server can observe
					// connection closure through its request context.
					if operation == "upload" {
						_, _ = io.Copy(io.Discard, r.Body)
					}
					if operation == "download" {
						fmt.Fprint(w, "partial")
						w.(http.Flusher).Flush()
					}
					close(started)
					select {
					case <-r.Context().Done():
						close(stopped)
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				dir := t.TempDir()
				file := filepath.Join(dir, "book.pdf")
				if err := os.WriteFile(file, []byte("book"), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{"projects", "list", "--json"}
				if operation == "upload" {
					args = []string{"files", "upload", "1", file}
				}
				if operation == "download" {
					args = []string{"files", "download", "1", "2", "-o", filepath.Join(dir, "download.pdf")}
				}
				child := exec.Command(binary, args...)
				child.Env = append(os.Environ(), "BOOKBEAM_HOST="+server.URL, "BOOKBEAM_TOKEN=fixture", "BOOKBEAM_CONFIG_DIR="+dir)
				var log bytes.Buffer
				child.Stdout, child.Stderr = &log, &log
				if err := child.Start(); err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() { done <- child.Wait() }()
				defer func() { _ = child.Process.Kill() }()
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					t.Fatal("request never started")
				}
				if err := child.Process.Signal(signal); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					exit, ok := err.(*exec.ExitError)
					if !ok || exit.ExitCode() != 1 || !bytes.Contains(log.Bytes(), []byte("Error:")) {
						t.Fatalf("want normal error exit, got %v: %s", err, log.String())
					}
				case <-time.After(5 * time.Second):
					t.Fatal("process did not stop")
				}
				select {
				case <-stopped:
				case <-time.After(5 * time.Second):
					t.Fatal("server did not observe cancellation")
				}
			})
		}
	}
}
