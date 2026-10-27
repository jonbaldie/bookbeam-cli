package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAPICommandsRequireAuthentication(t *testing.T) {
	commands := [][]string{
		{"whoami"}, {"auth", "whoami"},
		{"projects", "list"}, {"projects", "get", "1"},
		{"projects", "create", "--title", "Book"}, {"projects", "update", "1", "--title", "Book"},
		{"projects", "delete", "1"}, {"projects", "newsletter", "1"},
		{"files", "list", "1"}, {"files", "upload", "1", "missing.pdf"},
		{"files", "download", "1", "2"}, {"files", "delete", "1", "2"},
		{"links", "list", "1"}, {"links", "create", "1", "--title", "Launch"},
		{"links", "update", "1", "2", "--title", "Launch"}, {"links", "delete", "1", "2"},
		{"downloaders", "list", "1"}, {"downloaders", "export", "1"},
		{"newsletter", "status"}, {"newsletter", "configure"}, {"newsletter", "disconnect"},
		{"newsletter", "webhook", "--url", "https://example.com/hook"}, {"newsletter", "lists"},
		{"billing", "status"}, {"billing", "checkout"}, {"logs"}, {"metrics"},
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Unauthenticated."}`)
	}))
	defer server.Close()
	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, err := runCLIWithHome(t, t.TempDir(), append([]string{"--host", server.URL}, args...)...)
			if err == nil || out != "Error: Not authenticated. Run 'bookbeam auth login' to authenticate.\n" {
				t.Fatalf("expected only the login hint, got err=%v output=%q", err, out)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("missing credentials sent %d HTTP requests", got)
	}
}

func TestRejectedAuthenticationHasOneLoginHint(t *testing.T) {
	for _, args := range [][]string{
		{"projects", "list"}, {"files", "list", "1"}, {"downloaders", "list", "1"},
		{"whoami"}, {"auth", "whoami"}, {"logs"}, {"metrics"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "Bearer revoked-token" {
					requests.Add(1)
				}
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"message":"Unauthenticated."}`)
			}))
			defer server.Close()
			argv := append([]string{"--host", server.URL, "--token", "revoked-token"}, args...)
			out, err := runCLIWithHome(t, t.TempDir(), argv...)
			if err == nil || out != "Error: Not authenticated. Run 'bookbeam auth login' to authenticate.\n" || requests.Load() != 1 {
				t.Fatalf("expected a rejected authenticated request and only the hint, got requests=%d err=%v output=%q", requests.Load(), err, out)
			}
		})
	}
}

func TestAuthenticationExemptCommands(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "logout", "--quiet"}, {"completion", "bash"}, {"completion", "zsh"},
		{"completion", "fish"}, {"completion", "powershell"}, {"help"}, {"projects", "list", "--help"},
		{"__complete", "projects", ""}, {"__completeNoDesc", "projects", ""},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, err := runCLIWithHome(t, t.TempDir(), args...)
			if err != nil || strings.Contains(out, "Not authenticated") {
				t.Fatalf("exempt command failed: %v %q", err, out)
			}
		})
	}
	home := t.TempDir()
	out, err := runCLIWithHome(t, home, "auth", "login", "--token", "new-token", "--quiet")
	if err != nil || out != "" {
		t.Fatalf("direct login failed: %v %q", err, out)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "bookbeam", "config.json"))
	if err != nil || !strings.Contains(string(data), `"token": "new-token"`) {
		t.Fatalf("login did not save the new token: %v %s", err, data)
	}
}

func TestNonAuthenticationFailuresKeepTheirMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"Access denied."}`)
	}))
	defer server.Close()
	out, err := runCLIWithHome(t, t.TempDir(), "--host", server.URL, "--token", "valid-token", "projects", "list")
	if err == nil || !strings.Contains(out, "API error (403): Access denied.") || strings.Contains(out, "Not authenticated") {
		t.Fatalf("403 must remain distinct from authentication: %v %q", err, out)
	}
}
