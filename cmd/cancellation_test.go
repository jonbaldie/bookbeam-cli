package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCancelledDeviceWaitDoesNotPollOrSaveToken(t *testing.T) {
	isolateHome(t)
	t.Setenv("BOOKBEAM_TOKEN", "")
	codeSent := make(chan struct{})
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/oauth/device/code" {
			fmt.Fprint(w, `{"device_code":"fixture","expires_in":120,"interval":60}`)
			close(codeSent)
			return
		}
		polls.Add(1)
		fmt.Fprint(w, `{"access_token":"must-not-save"}`)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := NewRootCmd()
	root.SetArgs([]string{"--host", server.URL, "auth", "login", "--json"})
	result := make(chan error, 1)
	go func() { result <- root.ExecuteContext(ctx) }()
	select {
	case <-codeSent:
	case <-time.After(3 * time.Second):
		t.Fatal("device code not requested")
	}
	// Allow the code response to be consumed and the long polling wait to begin.
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("poll wait did not stop")
	}
	settings, err := config.Resolve(os.Getenv(config.ConfigDirEnv), func(string) string { return "" }, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if polls.Load() != 0 || settings.Token() != "" {
		t.Fatalf("polls=%d saved token=%q", polls.Load(), settings.Token())
	}
}

func TestActiveRemoteWorkCancellation(t *testing.T) {
	for _, operation := range []string{"short", "upload", "download", "error-response"} {
		t.Run(operation, func(t *testing.T) {
			isolateHome(t)
			file := filepath.Join(t.TempDir(), "book.pdf")
			if err := os.WriteFile(file, []byte("book"), 0600); err != nil {
				t.Fatal(err)
			}
			started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if operation == "upload" {
					_, _ = io.Copy(io.Discard, r.Body)
				}
				if operation == "error-response" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"error":"partial`)
					w.(http.Flusher).Flush()
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
			args := []string{"projects", "list", "--json"}
			if operation == "upload" {
				args = []string{"files", "upload", "1", file}
			}
			if operation == "download" {
				args = []string{"files", "download", "1", "2", "-o", filepath.Join(t.TempDir(), "download.pdf")}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := NewRootCmd()
			root.SetArgs(append([]string{"--host", server.URL, "--token", "fixture"}, args...))
			result := make(chan error, 1)
			go func() { result <- root.ExecuteContext(ctx) }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			if operation == "error-response" {
				time.Sleep(50 * time.Millisecond) // Let the client begin reading the unfinished error body.
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("want cancellation, got %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("command did not stop")
			}
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("server request was not cancelled")
			}
		})
	}
}

func TestCancelledLinkFetchPreventsMutation(t *testing.T) {
	isolateHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var puts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts.Add(1)
		}
		cancel()
		fmt.Fprint(w, `{"data":[{"id":2,"title":"before","opt_in_text":"preserve"}]}`)
	}))
	defer server.Close()
	root := NewRootCmd()
	root.SetArgs([]string{"--host", server.URL, "--token", "fixture", "links", "update", "1", "2", "--title", "after"})
	err := root.ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) || puts.Load() != 0 {
		t.Fatalf("err=%v PUTs=%d; want cancellation without mutation", err, puts.Load())
	}
}

func TestPreCancelledRootSendsNoRequest(t *testing.T) {
	isolateHome(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := NewRootCmd()
	root.SetArgs([]string{"--host", server.URL, "--token", "fixture", "projects", "list", "--json"})
	var err error
	out := arbCaptureStdout(t, func() { err = root.ExecuteContext(ctx) })
	if !errors.Is(err, context.Canceled) || requests.Load() != 0 || out != "" {
		t.Fatalf("err=%v requests=%d output=%q; want cancellation without remote work or success output", err, requests.Load(), out)
	}
}
