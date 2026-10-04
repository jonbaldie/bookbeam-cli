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

func assertCredentialsRedacted(t *testing.T, got string, sentinels []string, wantMasks int) {
	t.Helper()
	for _, sentinel := range sentinels {
		if strings.Contains(got, sentinel) {
			t.Errorf("JSON output exposed a credential sentinel: %s", sentinel)
		}
	}
	if count := strings.Count(got, `"********"`); count != wantMasks {
		t.Errorf("JSON output contains %d masked values, want %d: %s", count, wantMasks, got)
	}
	var document any
	if err := json.Unmarshal([]byte(got), &document); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestWhoamiJSONRedactsCredentialsAtOutputBoundary(t *testing.T) {
	const response = `{"name":"Test User","email":"test@example.com","current_team":{"name":"Team","newsletter_provider_config":{"api_token":"WHOAMI_TOKEN_SENTINEL","api_key":"WHOAMI_KEY_SENTINEL","authorization":"WHOAMI_AUTH_SENTINEL"}}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer:  &output.Printer{Out: &buf, JSON: true},
		settings: testSettings(t, ts.URL, "test-token"),
		apiCli:   client.New(ts.URL, "test-token"),
	}
	cmd := whoamiCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("run whoami: %v", err)
	}
	assertCredentialsRedacted(t, buf.String(), []string{"WHOAMI_TOKEN_SENTINEL", "WHOAMI_KEY_SENTINEL", "WHOAMI_AUTH_SENTINEL"}, 3)
}

func TestNewsletterStatusJSONRedactsCredentialsAtOutputBoundary(t *testing.T) {
	const response = `{"provider":"mailerlite","config":{"api_token":"STATUS_TOKEN_SENTINEL","api_url":"https://provider.example/api"},"session":{"password":"STATUS_PASSWORD_SENTINEL"}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/settings/newsletter" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer:  &output.Printer{Out: &buf, JSON: true},
		settings: testSettings(t, ts.URL, "test-token"),
		apiCli:   client.New(ts.URL, "test-token"),
	}
	cmd := newsletterStatusCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("run newsletter status: %v", err)
	}
	assertCredentialsRedacted(t, buf.String(), []string{"STATUS_TOKEN_SENTINEL", "STATUS_PASSWORD_SENTINEL"}, 2)
}

func TestNewsletterConfigureJSONRedactsResponseWithoutChangingRequest(t *testing.T) {
	const requestSecret = "CONFIGURE_REQUEST_SENTINEL"
	const responseSecret = "CONFIGURE_RESPONSE_SENTINEL"
	var request map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/settings/newsletter/provider" || r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode configure request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"provider":"mailcoach","config":{"api_token":"` + responseSecret + `"}}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer:  &output.Printer{Out: &buf, JSON: true},
		settings: testSettings(t, ts.URL, "test-token"),
		apiCli:   client.New(ts.URL, "test-token"),
	}
	cmd := newsletterConfigureCmd(a)
	if err := cmd.Flags().Set("provider", "mailcoach"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("api-key", requestSecret); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("run newsletter configure: %v", err)
	}
	if got := request["api_token"]; got != requestSecret {
		t.Errorf("outgoing API token changed: got %v", got)
	}
	assertCredentialsRedacted(t, buf.String(), []string{responseSecret}, 1)
}
