package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestDirectTokenLoginAndLogout(t *testing.T) {
	a := &app{}
	loginCmd := loginCmd(a)
	logoutCmd := logoutCmd(a)

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf}
	a.settings = testSettings(t, "http://localhost:8000", "")
	_ = loginCmd.Flags().Set("token", "direct-token-abc")

	err := loginCmd.RunE(loginCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	if !strings.Contains(buf.String(), "saved successfully") {
		t.Errorf("expected success message, got %s", buf.String())
	}

	if a.settings.Token() != "direct-token-abc" {
		t.Errorf("expected token direct-token-abc, got %s", a.settings.Token())
	}

	buf.Reset()
	err = logoutCmd.RunE(logoutCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected logout error: %v", err)
	}

	if !strings.Contains(buf.String(), "Logged out successfully") {
		t.Errorf("expected logout message, got %s", buf.String())
	}

	if a.settings.Token() != "" {
		t.Errorf("expected empty token after logout, got %s", a.settings.Token())
	}
}

func TestWhoamiCommand(t *testing.T) {
	a := &app{}
	whoamiCmd := whoamiCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    42,
			"name":  "Jonathan Baldie",
			"email": "jon@example.com",
			"current_team": map[string]any{
				"id":   1,
				"name": "Subject Zero",
			},
		})
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.settings = testSettings(t, ts.URL, "valid-token")
	a.apiCli = client.New(ts.URL, "valid-token")

	err := whoamiCmd.RunE(whoamiCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected whoami error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Jonathan Baldie") {
		t.Errorf("expected output to contain Jonathan Baldie, got %s", out)
	}
	if !strings.Contains(out, "Subject Zero") {
		t.Errorf("expected output to contain Subject Zero, got %s", out)
	}
}
