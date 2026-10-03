package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

func TestRedactNewsletterProviderCredentialsPreservesOtherDocuments(t *testing.T) {
	teamWithoutConfig := map[string]any{"current_team": map[string]any{"name": "Team"}}
	emptyCredentials := map[string]any{
		"current_team": map[string]any{
			"newsletter_provider_config": map[string]any{
				"api_token": "",
				"api_key":   7,
				"api_url":   "https://provider.example/api",
			},
		},
	}
	emptyCredentialsWant := map[string]any{
		"current_team": map[string]any{
			"newsletter_provider_config": map[string]any{
				"api_token": "",
				"api_key":   7,
				"api_url":   "https://provider.example/api",
			},
		},
	}

	for _, tc := range []struct {
		name string
		doc  any
		want any
	}{
		{name: "non-object response", doc: "plain response", want: "plain response"},
		{name: "no current team", doc: map[string]any{"name": "Author"}, want: map[string]any{"name": "Author"}},
		{name: "no provider config", doc: teamWithoutConfig, want: teamWithoutConfig},
		{name: "empty or non-string credentials", doc: emptyCredentials, want: emptyCredentialsWant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactNewsletterProviderCredentials(tc.doc); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

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
				"newsletter_provider_config": map[string]any{
					"api_token": "synthetic-provider-token",
				},
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
	if strings.Contains(out, "synthetic-provider-token") {
		t.Error("human-readable whoami output exposed the newsletter provider credential")
	}
}

func TestWhoamiJSONMasksNewsletterProviderCredentials(t *testing.T) {
	const response = `{"name":"Jonathan Baldie","email":"jon@example.com","current_team":{"name":"Subject Zero","newsletter_provider_config":{"api_token":"synthetic-provider-token","api_key":"synthetic-provider-key","api_url":"https://provider.example/api"}}}`
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
		_, _ = w.Write([]byte(response))
	}))
	defer ts.Close()

	for _, tc := range []struct {
		name   string
		nested bool
	}{
		{name: "whoami"},
		{name: "auth whoami", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &app{}
			var buf bytes.Buffer
			a.printer = &output.Printer{Out: &buf, JSON: true}
			a.settings = testSettings(t, ts.URL, "valid-token")
			a.apiCli = client.New(ts.URL, "valid-token")

			command := whoamiCmd(a)
			if tc.nested {
				var err error
				command, _, err = authCmd(a).Find([]string{"whoami"})
				if err != nil {
					t.Fatalf("find auth whoami command: %v", err)
				}
			}
			if err := command.RunE(command, nil); err != nil {
				t.Fatalf("run whoami: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("whoami output is not valid JSON: %v", err)
			}
			if got["name"] != "Jonathan Baldie" || got["email"] != "jon@example.com" {
				t.Fatalf("whoami JSON lost non-secret user data: %#v", got)
			}
			team, ok := got["current_team"].(map[string]any)
			if !ok || team["name"] != "Subject Zero" {
				t.Fatalf("whoami JSON lost non-secret team data: %#v", got["current_team"])
			}
			providerConfig, ok := team["newsletter_provider_config"].(map[string]any)
			if !ok {
				t.Fatalf("whoami JSON lost provider configuration: %#v", team["newsletter_provider_config"])
			}
			for _, key := range []string{"api_token", "api_key"} {
				if value := providerConfig[key]; value != "********" {
					t.Errorf("%s = %#v, want masked value", key, value)
				}
			}
			if providerConfig["api_url"] != "https://provider.example/api" {
				t.Errorf("whoami JSON lost non-secret provider data: %#v", providerConfig["api_url"])
			}
			if strings.Contains(buf.String(), "synthetic-provider-") {
				t.Error("whoami JSON exposed a newsletter provider credential")
			}
		})
	}
}
