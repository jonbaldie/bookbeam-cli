package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestNewsletterTelemetryBillingCommands(t *testing.T) {
	a := &app{}
	billingStatusCmd := billingStatusCmd(a)
	logsCmd := logsCmd(a)
	metricsCmd := metricsCmd(a)
	newsletterStatusCmd := newsletterStatusCmd(a)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/settings/newsletter" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"provider":    "mailerlite",
				"webhook_url": "https://hooks.zapier.com/test",
				"config": map[string]any{
					"api_token": "********",
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/logs" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"current_page": 1,
				"per_page":     15,
				"total":        1,
				"last_page":    1,
				"data": []map[string]any{
					{
						"id":               1,
						"type":             "signup",
						"occurred_at":      "2026-09-13T12:00:00Z",
						"reader_email":     "reader@example.com",
						"book_title":       "Reader Magnet Playbook",
						"signup_link_slug": "iNS2rgt9RS",
					},
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/dashboard/metrics" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_projects":  10,
				"total_files":     25,
				"total_downloads": 400,
				"total_views":     1200,
			})
			return
		}

		if r.URL.Path == "/api/v1/billing" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"has_access": true,
				"subscription": map[string]any{
					"active": true,
					"name":   "Pro Monthly",
					"status": "active",
				},
				"active_offer": map[string]any{
					"name":         "monthly",
					"price_label":  "$19/month",
					"checkout_url": "https://bookbeam.lemonsqueezy.com/checkout/buy/123",
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a.printer = &output.Printer{Out: &buf, JSON: false}
	a.cfg = &config.Config{Host: ts.URL, Token: "test-token"}
	a.apiCli = client.New(ts.URL, "test-token")

	err := newsletterStatusCmd.RunE(newsletterStatusCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected newsletter status error: %v", err)
	}
	if !strings.Contains(buf.String(), "MAILERLITE") {
		t.Errorf("expected newsletter status to show MAILERLITE, got %s", buf.String())
	}

	buf.Reset()
	err = logsCmd.RunE(logsCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected logs error: %v", err)
	}
	if !strings.Contains(buf.String(), "reader@example.com") {
		t.Errorf("expected logs to contain the reader email, got %s", buf.String())
	}

	buf.Reset()
	err = metricsCmd.RunE(metricsCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected metrics error: %v", err)
	}
	if !strings.Contains(buf.String(), "400") {
		t.Errorf("expected metrics to contain 400 downloads, got %s", buf.String())
	}

	buf.Reset()
	err = billingStatusCmd.RunE(billingStatusCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected billing status error: %v", err)
	}
	if !strings.Contains(buf.String(), "Paid access granted") {
		t.Errorf("expected billing status to show 'Paid access granted', got %s", buf.String())
	}
}

func TestNewsletterStatusUsesProviderAndToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/settings/newsletter" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"provider":"mailcoach","config":{"api_url":"https://mailcoach.example/api","api_token":"********","default_list_id":null},"webhook_url":null}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer: &output.Printer{Out: &buf, JSON: false},
		cfg:     &config.Config{Host: ts.URL, Token: "test-token"},
		apiCli:  client.New(ts.URL, "test-token"),
	}
	cmd := newsletterStatusCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected newsletter status error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "MAILCOACH") {
		t.Errorf("expected provider MAILCOACH, got %s", got)
	}
	if !strings.Contains(got, "Configured         Yes") {
		t.Errorf("expected Configured: Yes for a working provider, got %s", got)
	}
	if !strings.Contains(got, "Webhook Endpoint   None") {
		t.Errorf("expected Webhook Endpoint None, got %s", got)
	}
}

func TestNewsletterListsUsesDataArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/settings/newsletter/lists" && r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":[{"id":"list-1","name":"Launch list"},{"id":"list-2","name":"Readers"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer: &output.Printer{Out: &buf, JSON: false},
		cfg:     &config.Config{Host: ts.URL, Token: "test-token"},
		apiCli:  client.New(ts.URL, "test-token"),
	}
	cmd := newsletterListsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected newsletter lists error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "No mailing lists") {
		t.Errorf("expected lists table, got empty message: %s", got)
	}
	if !strings.Contains(got, "list-1") || !strings.Contains(got, "Launch list") {
		t.Errorf("expected list entries, got %s", got)
	}
	if strings.Contains(got, "Tags:") || strings.Contains(got, "TAG ID") {
		t.Errorf("expected no tags section, got %s", got)
	}
}

// newsletterProviderValidator mimics NewsletterSettingsController::updateProvider,
// which requires api_token and treats api_url as the optional endpoint override.
func newsletterProviderValidator(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	var validated map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if token, ok := body["api_token"].(string); !ok || token == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"The api token field is required."}`))
			return
		}
		validated = body
		_, _ = w.Write([]byte(`{"provider":"mailcoach"}`))
	}))
	t.Cleanup(ts.Close)
	return ts, &validated
}

func TestNewsletterConfigureMatchesProviderAPIContract(t *testing.T) {
	ts, validated := newsletterProviderValidator(t)
	var buf bytes.Buffer
	a := &app{
		printer: &output.Printer{Out: &buf, JSON: false},
		cfg:     &config.Config{Host: ts.URL, Token: "test-token"},
		apiCli:  client.New(ts.URL, "test-token"),
	}
	cmd := newsletterConfigureCmd(a)
	_ = cmd.Flags().Set("provider", "MailCoach")
	_ = cmd.Flags().Set("api-key", "mc_secret_key")
	_ = cmd.Flags().Set("endpoint", "https://mailcoach.example.com")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("configure failed: %v", err)
	}
	got := *validated
	if got["provider"] != "mailcoach" {
		t.Errorf("provider: got %v, want mailcoach", got["provider"])
	}
	if got["api_token"] != "mc_secret_key" {
		t.Errorf("api_token: got %v, want mc_secret_key", got["api_token"])
	}
	if got["api_url"] != "https://mailcoach.example.com" {
		t.Errorf("api_url: got %v, want https://mailcoach.example.com", got["api_url"])
	}
	if len(got) != 3 {
		t.Errorf("unexpected extra payload fields: %v", got)
	}
}

func TestNewsletterConfigureOmitsAPIURLWithoutEndpoint(t *testing.T) {
	ts, validated := newsletterProviderValidator(t)
	var buf bytes.Buffer
	a := &app{
		printer: &output.Printer{Out: &buf, JSON: false},
		cfg:     &config.Config{Host: ts.URL, Token: "test-token"},
		apiCli:  client.New(ts.URL, "test-token"),
	}
	cmd := newsletterConfigureCmd(a)
	_ = cmd.Flags().Set("provider", "mailerlite")
	_ = cmd.Flags().Set("api-key", "ml_secret_key")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("configure failed: %v", err)
	}
	got := *validated
	if _, ok := got["api_url"]; ok {
		t.Errorf("expected no api_url, got %v", got)
	}
	if got["api_token"] != "ml_secret_key" || len(got) != 2 {
		t.Errorf("unexpected body %v", got)
	}
}
