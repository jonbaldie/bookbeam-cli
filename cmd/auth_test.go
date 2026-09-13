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

func setupTestEnv(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "bookbeam-cmd-test-*")
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tempDir, "config.json")
	_ = config.Save(&config.Config{Host: "http://localhost:8000"}, configPath)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)

	return tempDir, func() {
		os.Setenv("HOME", oldHome)
		_ = os.RemoveAll(tempDir)
	}
}

func TestDirectTokenLoginAndLogout(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	var buf bytes.Buffer
	printer = &output.Printer{Out: &buf, Err: &buf}
	cfg = &config.Config{Host: "http://localhost:8000"}
	flagDirectToken = "direct-token-abc"

	err := loginCmd.RunE(loginCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	if !strings.Contains(buf.String(), "saved successfully") {
		t.Errorf("expected success message, got %s", buf.String())
	}

	if cfg.Token != "direct-token-abc" {
		t.Errorf("expected token direct-token-abc, got %s", cfg.Token)
	}

	buf.Reset()
	err = logoutCmd.RunE(logoutCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected logout error: %v", err)
	}

	if !strings.Contains(buf.String(), "Logged out successfully") {
		t.Errorf("expected logout message, got %s", buf.String())
	}

	if cfg.Token != "" {
		t.Errorf("expected empty token after logout, got %s", cfg.Token)
	}
}

func TestWhoamiCommand(t *testing.T) {
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
	printer = &output.Printer{Out: &buf, Err: &buf, JSON: false}
	cfg = &config.Config{Host: ts.URL, Token: "valid-token"}
	apiCli = client.New(ts.URL, "valid-token")

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
