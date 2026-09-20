package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

// ntdRequest is what the fake API saw for one call.
type ntdRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

// ntdServer answers every request with status and body, recording what it received.
func ntdServer(t *testing.T, status int, body string) (*httptest.Server, *[]ntdRequest) {
	t.Helper()
	var seen []ntdRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = append(seen, ntdRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(raw)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, &seen
}

func ntdApp(serverURL string, jsonOutput bool) (*app, *bytes.Buffer) {
	var buf bytes.Buffer
	return &app{
		cfg:     &config.Config{Host: serverURL, Token: "tok"},
		apiCli:  client.New(serverURL, "tok"),
		printer: &output.Printer{Out: &buf, JSON: jsonOutput},
	}, &buf
}

// ntdRoot runs the real command tree and returns its help/usage output.
func ntdRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BOOKBEAM_HOST", "")
	t.Setenv("BOOKBEAM_TOKEN", "")
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func ntdOnly(t *testing.T, seen *[]ntdRequest) ntdRequest {
	t.Helper()
	if len(*seen) != 1 {
		t.Fatalf("expected exactly one request, got %+v", *seen)
	}
	return (*seen)[0]
}

func ntdJSONBody(t *testing.T, body string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("request body is not JSON: %q", body)
	}
	return got
}

func TestNtdHelpListsCommandsAndDescriptions(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"downloaders", "--help"}, []string{
			"View reader downloaders and export subscriber lists",
			"list        List downloader subscribers for a book project",
			"export      Export sanitized CSV list of downloader subscribers",
			"Aliases:\n  downloaders, downloader",
		}},
		{[]string{"downloaders", "list", "--help"}, []string{"bookbeam downloaders list <project-id> [flags]", "--page int"}},
		{[]string{"downloaders", "export", "--help"}, []string{"bookbeam downloaders export <project-id> [flags]", "-o, --output string"}},
		{[]string{"newsletter", "--help"}, []string{
			"Manage mailing list integrations and webhook notifications",
			"configure   Connect a newsletter service (mailerlite, kit, mailcoach)",
			"disconnect  Disconnect active newsletter provider credentials",
			"lists       Fetch available lists from configured provider",
			"status      Show current team newsletter provider settings",
			"webhook     Set or clear subscriber notification webhook URL",
		}},
		{[]string{"newsletter", "configure", "--help"}, []string{"--api-key string", "--endpoint string", "--provider string"}},
		{[]string{"newsletter", "webhook", "--help"}, []string{"--url string"}},
		{[]string{"--help"}, []string{
			"logs        View chronological telemetry activity logs",
			"metrics     Display aggregated catalog and reader engagement metrics",
		}},
		{[]string{"logs", "--help"}, []string{"-n, --limit int", "(default 50)", "--event string", "capped at 100", "--json stays unfiltered"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			out, err := ntdRoot(t, tc.args...)
			if err != nil {
				t.Fatalf("help failed: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("help output missing %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestNtdDownloaderCommandsRequireOneProjectID(t *testing.T) {
	for _, sub := range []string{"list", "export"} {
		for _, args := range [][]string{{}, {"1", "2"}} {
			_, err := ntdRoot(t, append([]string{"downloaders", sub}, args...)...)
			if err == nil || !strings.Contains(err.Error(), "accepts 1 arg(s)") {
				t.Errorf("downloaders %s %v: expected arg count error, got %v", sub, args, err)
			}
		}
	}
}

func TestNtdDownloadersListRequestsPageAndPrintsTable(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"data":[{"id":1,"email":"a@example.com","signup_link_title":"Promo","signed_up_at":"2026-01-01"},{"id":2,"email":"b@example.com","signup_link_title":"Blog","signed_up_at":"2026-01-02"}],"current_page":2,"last_page":3,"total":7}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := downloadersListCmd(a)

	if err := cmd.RunE(cmd, []string{"9"}); err != nil {
		t.Fatal(err)
	}
	req := ntdOnly(t, seen)
	if req.Method != "GET" || req.Path != "/api/v1/projects/9/downloaders" || req.Query != "page=1" {
		t.Errorf("unexpected request %+v", req)
	}
	want := "EMAIL           SIGNUP LINK   SIGNUP DATE\n" +
		"a@example.com   Promo         2026-01-01\n" +
		"b@example.com   Blog          2026-01-02\n" +
		"\nPage 2 of 3 (Total: 7)\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestNtdDownloadersListPageFlag(t *testing.T) {
	cases := map[string]string{"4": "page=4", "0": "", "-1": ""}
	for page, query := range cases {
		ts, seen := ntdServer(t, 200, `{"data":[]}`)
		a, _ := ntdApp(ts.URL, false)
		cmd := downloadersListCmd(a)
		_ = cmd.Flags().Set("page", page)
		if err := cmd.RunE(cmd, []string{"9"}); err != nil {
			t.Fatal(err)
		}
		if got := ntdOnly(t, seen).Query; got != query {
			t.Errorf("page %s: query %q, want %q", page, got, query)
		}
	}
}

func TestNtdDownloadersListEmptyJSONAndErrors(t *testing.T) {
	ts, _ := ntdServer(t, 200, `{"data":[],"current_page":1,"last_page":1,"total":0}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := downloadersListCmd(a)
	if err := cmd.RunE(cmd, []string{"9"}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "No downloaders found for this project.\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `{"data":[{"email":"x"}]}`)
	a, buf = ntdApp(ts.URL, true)
	cmd = downloadersListCmd(a)
	if err := cmd.RunE(cmd, []string{"9"}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"data\": [\n    {\n      \"email\": \"x\"\n    }\n  ]\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `not json`)
	a, buf = ntdApp(ts.URL, false)
	cmd = downloadersListCmd(a)
	if err := cmd.RunE(cmd, []string{"9"}); err == nil {
		t.Error("expected parse error")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 403, `{"message":"Forbidden"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = downloadersListCmd(a)
	if err := cmd.RunE(cmd, []string{"9"}); err == nil || err.Error() != "API error (403): Forbidden" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdDownloadersExportWritesCSV(t *testing.T) {
	ts, seen := ntdServer(t, 200, "Email\nx@example.com\n")
	a, buf := ntdApp(ts.URL, false)
	cmd := downloadersExportCmd(a)
	dest := filepath.Join(t.TempDir(), "out.csv")
	_ = cmd.Flags().Set("output", dest)

	if err := cmd.RunE(cmd, []string{"12"}); err != nil {
		t.Fatal(err)
	}
	req := ntdOnly(t, seen)
	if req.Method != "GET" || req.Path != "/api/v1/projects/12/export-downloaders" || req.Query != "" {
		t.Errorf("unexpected request %+v", req)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "Email\nx@example.com\n" {
		t.Fatalf("file content %q, err %v", data, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(dest)
		if info.Mode().Perm() != 0644 {
			t.Errorf("file mode %v, want 0644", info.Mode().Perm())
		}
	}
	if want := "✓ Exported 20 bytes to " + dest + "\n"; buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestNtdDownloadersExportDefaultPath(t *testing.T) {
	ts, _ := ntdServer(t, 200, "csv")
	a, buf := ntdApp(ts.URL, false)
	cmd := downloadersExportCmd(a)

	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := cmd.RunE(cmd, []string{"12"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "downloaders-project-12.csv"))
	if err != nil || string(data) != "csv" {
		t.Fatalf("file content %q, err %v", data, err)
	}
	if buf.String() != "✓ Exported 3 bytes to downloaders-project-12.csv\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestNtdDownloadersExportErrors(t *testing.T) {
	ts, _ := ntdServer(t, 404, `{"message":"Not found"}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := downloadersExportCmd(a)
	dest := filepath.Join(t.TempDir(), "out.csv")
	_ = cmd.Flags().Set("output", dest)
	if err := cmd.RunE(cmd, []string{"12"}); err == nil || err.Error() != "API error (404): Not found" {
		t.Errorf("expected API error, got %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("expected no file written, stat err %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, "csv")
	a, buf = ntdApp(ts.URL, false)
	cmd = downloadersExportCmd(a)
	_ = cmd.Flags().Set("output", filepath.Join(t.TempDir(), "missing", "out.csv"))
	err := cmd.RunE(cmd, []string{"12"})
	if err == nil || !strings.HasPrefix(err.Error(), "failed to save CSV file: ") {
		t.Errorf("expected save error, got %v", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected wrapped not-exist error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdNewsletterStatusText(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"configured", `{"provider":"kit","webhook_url":"https://example.com/hook","config":{"api_token":"********"}}`,
			"Setting            Value\nActive Provider    KIT\nConfigured         Yes\nWebhook Endpoint   https://example.com/hook\n"},
		{"mailcoach live", `{"provider":"mailcoach","config":{"api_url":"https://mailcoach.example/api","api_token":"********","default_list_id":null},"webhook_url":null}`,
			"Setting            Value\nActive Provider    MAILCOACH\nConfigured         Yes\nWebhook Endpoint   None\n"},
		{"disconnected", `{"provider":"","webhook_url":"","config":{"api_token":""}}`,
			"Setting            Value\nActive Provider    NONE (DISCONNECTED)\nConfigured         No\nWebhook Endpoint   None\n"},
		{"provider without token", `{"provider":"mailcoach","config":{"api_token":""},"webhook_url":null}`,
			"Setting            Value\nActive Provider    MAILCOACH\nConfigured         No\nWebhook Endpoint   None\n"},
		{"token without provider", `{"provider":"","config":{"api_token":"********"},"webhook_url":null}`,
			"Setting            Value\nActive Provider    NONE (DISCONNECTED)\nConfigured         No\nWebhook Endpoint   None\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, seen := ntdServer(t, 200, tc.body)
			a, buf := ntdApp(ts.URL, false)
			cmd := newsletterStatusCmd(a)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			req := ntdOnly(t, seen)
			if req.Method != "GET" || req.Path != "/api/v1/settings/newsletter" || req.Query != "" {
				t.Errorf("unexpected request %+v", req)
			}
			if buf.String() != tc.want {
				t.Errorf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestNtdNewsletterStatusJSONAndErrors(t *testing.T) {
	ts, _ := ntdServer(t, 200, `{"provider":"kit"}`)
	a, buf := ntdApp(ts.URL, true)
	cmd := newsletterStatusCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"provider\": \"kit\"\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `[1]`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterStatusCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("expected parse error")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 500, `{"message":"Boom"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterStatusCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (500): Boom" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdNewsletterConfigureValidation(t *testing.T) {
	cases := []struct {
		provider, apiKey, want string
	}{
		{"", "key", "--provider is required (kit, mailcoach, or mailerlite)"},
		{"kit", "", "--api-key is required"},
	}
	for _, tc := range cases {
		ts, seen := ntdServer(t, 200, `{}`)
		a, _ := ntdApp(ts.URL, false)
		cmd := newsletterConfigureCmd(a)
		_ = cmd.Flags().Set("provider", tc.provider)
		_ = cmd.Flags().Set("api-key", tc.apiKey)
		err := cmd.RunE(cmd, nil)
		if err == nil || err.Error() != tc.want {
			t.Errorf("got %v, want %q", err, tc.want)
		}
		if len(*seen) != 0 {
			t.Errorf("expected no request, got %+v", *seen)
		}
	}
}

func TestNtdNewsletterConfigureSendsProvider(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"provider":"mailcoach"}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := newsletterConfigureCmd(a)
	_ = cmd.Flags().Set("provider", "MailCoach")
	_ = cmd.Flags().Set("api-key", "secret")
	_ = cmd.Flags().Set("endpoint", "https://example.com/api")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	req := ntdOnly(t, seen)
	if req.Method != "PUT" || req.Path != "/api/v1/settings/newsletter/provider" {
		t.Errorf("unexpected request %+v", req)
	}
	body := ntdJSONBody(t, req.Body)
	if len(body) != 3 || body["provider"] != "mailcoach" || body["api_key"] != "secret" || body["api_endpoint"] != "https://example.com/api" {
		t.Errorf("unexpected body %v", body)
	}
	if buf.String() != "✓ Connected provider: MAILCOACH\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestNtdNewsletterConfigureWithoutEndpointJSONAndError(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"provider":"kit"}`)
	a, buf := ntdApp(ts.URL, true)
	cmd := newsletterConfigureCmd(a)
	_ = cmd.Flags().Set("provider", "kit")
	_ = cmd.Flags().Set("api-key", "secret")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	body := ntdJSONBody(t, ntdOnly(t, seen).Body)
	if _, ok := body["api_endpoint"]; ok || len(body) != 2 {
		t.Errorf("unexpected body %v", body)
	}
	if buf.String() != "{\n  \"provider\": \"kit\"\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 422, `{"message":"Invalid key"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterConfigureCmd(a)
	_ = cmd.Flags().Set("provider", "kit")
	_ = cmd.Flags().Set("api-key", "bad")
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (422): Invalid key" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdNewsletterDisconnect(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"message":"ok"}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := newsletterDisconnectCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	req := ntdOnly(t, seen)
	if req.Method != "DELETE" || req.Path != "/api/v1/settings/newsletter/provider" {
		t.Errorf("unexpected request %+v", req)
	}
	if buf.String() != "✓ Disconnected newsletter provider.\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `{"message":"ok"}`)
	a, buf = ntdApp(ts.URL, true)
	cmd = newsletterDisconnectCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"message\": \"ok\"\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 500, `{"message":"Boom"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterDisconnectCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (500): Boom" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdNewsletterWebhook(t *testing.T) {
	cases := []struct {
		url, want string
	}{
		{"https://example.com/hook", "✓ Configured webhook URL: https://example.com/hook\n"},
		{"", "✓ Cleared newsletter webhook URL.\n"},
	}
	for _, tc := range cases {
		ts, seen := ntdServer(t, 200, `{}`)
		a, buf := ntdApp(ts.URL, false)
		cmd := newsletterWebhookCmd(a)
		_ = cmd.Flags().Set("url", tc.url)
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
		req := ntdOnly(t, seen)
		if req.Method != "PUT" || req.Path != "/api/v1/settings/newsletter/webhook" {
			t.Errorf("unexpected request %+v", req)
		}
		body := ntdJSONBody(t, req.Body)
		if v, ok := body["webhook_url"]; !ok || v != tc.url || len(body) != 1 {
			t.Errorf("unexpected body %v", body)
		}
		if buf.String() != tc.want {
			t.Errorf("got %q, want %q", buf.String(), tc.want)
		}
	}

	ts, _ := ntdServer(t, 200, `{"webhook_url":null}`)
	a, buf := ntdApp(ts.URL, true)
	cmd := newsletterWebhookCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"webhook_url\": null\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 422, `{"message":"Bad URL"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterWebhookCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (422): Bad URL" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdNewsletterListsText(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"lists", `{"data":[{"id":"l1","name":"Main"},{"id":"l22","name":"VIP"}]}`,
			"Mailing Lists:\nLIST ID   NAME\nl1        Main\nl22       VIP\n"},
		{"one list", `{"data":[{"id":"l1","name":"Main"}]}`,
			"Mailing Lists:\nLIST ID   NAME\nl1        Main\n"},
		{"ignores tags", `{"data":[{"id":"l1","name":"Main"}],"tags":[{"id":"t1","name":"Fans"}]}`,
			"Mailing Lists:\nLIST ID   NAME\nl1        Main\n"},
		{"none", `{"data":[]}`,
			"No mailing lists returned by provider.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, seen := ntdServer(t, 200, tc.body)
			a, buf := ntdApp(ts.URL, false)
			cmd := newsletterListsCmd(a)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			req := ntdOnly(t, seen)
			if req.Method != "POST" || req.Path != "/api/v1/settings/newsletter/lists" || req.Body != "{}" {
				t.Errorf("unexpected request %+v", req)
			}
			if buf.String() != tc.want {
				t.Errorf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestNtdNewsletterListsJSONAndErrors(t *testing.T) {
	ts, _ := ntdServer(t, 200, `{"lists":[]}`)
	a, buf := ntdApp(ts.URL, true)
	cmd := newsletterListsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"lists\": []\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `nope`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterListsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("expected parse error")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 400, `{"message":"Not configured"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = newsletterListsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (400): Not configured" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

// ntdLogsPage is a redacted capture of GET /api/v1/logs in production (issue #1):
// a Laravel paginator whose events sit under data.
const ntdLogsPage = `{"current_page":1,"data":[` +
	`{"id":5,"type":"download","occurred_at":"2026-09-15T08:00:01.000000Z",` +
	`"reader_email":"reader@example.com","book_title":"Reader Magnet Playbook",` +
	`"filename":"the-reader-magnet-playbook.pdf","signup_link_slug":"iNS2rgt9RS"},` +
	`{"id":6,"type":"signup","occurred_at":"2026-09-15T09:10:00.000000Z",` +
	`"reader_email":"other@example.com","book_title":"Reader Magnet Playbook",` +
	`"filename":null,"signup_link_slug":"iNS2rgt9RS","webhook_status":null,` +
	`"webhook_success":null,"newsletter_provider":"mailcoach",` +
	`"newsletter_provider_success":true}],` +
	`"per_page":15,"total":11,"last_page":1,"first_page_url":"https://bookbeam.app/api/v1/logs?page=1"}`

func TestNtdLogsQueryFlags(t *testing.T) {
	cases := []struct {
		name, limit, event, query string
	}{
		{"defaults", "", "", "per_page=50"},
		{"limit and event", "5", "signup", "per_page=5"},
		{"zero limit omitted", "0", "", ""},
		{"negative limit omitted", "-3", "download", ""},
		{"limit one", "1", "", "per_page=1"},
		{"limit at cap", "100", "", "per_page=100"},
		{"limit above cap", "101", "", "per_page=100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, seen := ntdServer(t, 200, `{"data":[]}`)
			a, _ := ntdApp(ts.URL, false)
			cmd := logsCmd(a)
			if tc.limit != "" {
				_ = cmd.Flags().Set("limit", tc.limit)
			}
			_ = cmd.Flags().Set("event", tc.event)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			req := ntdOnly(t, seen)
			if req.Method != "GET" || req.Path != "/api/v1/logs" || req.Query != tc.query {
				t.Errorf("unexpected request %+v, want query %q", req, tc.query)
			}
		})
	}
}

// TestNtdLogsProductionShape pins the table against the captured paginator.
func TestNtdLogsProductionShape(t *testing.T) {
	ts, _ := ntdServer(t, 200, ntdLogsPage)
	a, buf := ntdApp(ts.URL, false)
	cmd := logsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logs failed on the production shape: %v", err)
	}
	want := "TIME                          TYPE       READER               BOOK                     FILE\n" +
		"2026-09-15T08:00:01.000000Z   download   reader@example.com   Reader Magnet Playbook   the-reader-magnet-playbook.pdf\n" +
		"2026-09-15T09:10:00.000000Z   signup     other@example.com    Reader Magnet Playbook   \n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// TestNtdLogsEventFilter covers the client-side filter: the API has no event param.
func TestNtdLogsEventFilter(t *testing.T) {
	cases := []struct {
		name, event string
		wantRows    []string
		wantAbsent  []string
	}{
		{"no filter keeps both", "", []string{"download", "signup"}, nil},
		{"signup only", "signup", []string{"signup"}, []string{"download"}},
		{"download only", "download", []string{"download"}, []string{"signup"}},
		{"case insensitive", "SignUp", []string{"signup"}, []string{"download"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := ntdServer(t, 200, ntdLogsPage)
			a, buf := ntdApp(ts.URL, false)
			cmd := logsCmd(a)
			_ = cmd.Flags().Set("event", tc.event)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.wantRows {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("expected %q in %q", want, buf.String())
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(buf.String(), absent) {
					t.Errorf("expected no %q in %q", absent, buf.String())
				}
			}
		})
	}

	ts, _ := ntdServer(t, 200, ntdLogsPage)
	a, buf := ntdApp(ts.URL, false)
	cmd := logsCmd(a)
	_ = cmd.Flags().Set("event", "refund")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "No activity logs recorded.\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestNtdLogsOutput(t *testing.T) {
	ts, _ := ntdServer(t, 200, `{"data":[]}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := logsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "No activity logs recorded.\n" {
		t.Errorf("got %q", buf.String())
	}

	// --json hands back the paginator untouched, filter or no filter.
	ts, _ = ntdServer(t, 200, `{"data":[{"id":1}]}`)
	a, buf = ntdApp(ts.URL, true)
	cmd = logsCmd(a)
	_ = cmd.Flags().Set("event", "signup")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"data\": [\n    {\n      \"id\": 1\n    }\n  ]\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `[{"id":1}]`)
	a, buf = ntdApp(ts.URL, false)
	cmd = logsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("expected parse error")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 401, `{"message":"Unauthenticated."}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = logsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (401): Unauthenticated." {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestNtdMetrics(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"total_projects":3,"total_files":14,"total_downloads":159,"total_views":2653}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := metricsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	req := ntdOnly(t, seen)
	if req.Method != "GET" || req.Path != "/api/v1/dashboard/metrics" || req.Query != "" {
		t.Errorf("unexpected request %+v", req)
	}
	want := "Metric                     Total\n" +
		"Total Book Projects        3\n" +
		"Total Uploaded Files       14\n" +
		"Total Reader Downloads     159\n" +
		"Total Landing Page Views   2653\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}

	ts, _ = ntdServer(t, 200, `{"total_projects":3}`)
	a, buf = ntdApp(ts.URL, true)
	cmd = metricsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\n  \"total_projects\": 3\n}\n" {
		t.Errorf("got %q", buf.String())
	}

	ts, _ = ntdServer(t, 200, `{"total_projects":"many"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = metricsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("expected parse error")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}

	ts, _ = ntdServer(t, 500, `{"message":"Boom"}`)
	a, buf = ntdApp(ts.URL, false)
	cmd = metricsCmd(a)
	if err := cmd.RunE(cmd, nil); err == nil || err.Error() != "API error (500): Boom" {
		t.Errorf("expected API error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}
