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
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":         1,
					"type":       "signup",
					"summary":    "Reader signed up",
					"status":     "success",
					"created_at": "2026-09-13T12:00:00Z",
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
				"has_access":   true,
				"offer_type":   "subscription",
				"active_offer": "monthly",
				"checkout_url": "https://bookbeam.lemonsqueezy.com/checkout/buy/123",
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
	if !strings.Contains(buf.String(), "Reader signed up") {
		t.Errorf("expected logs to contain 'Reader signed up', got %s", buf.String())
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
