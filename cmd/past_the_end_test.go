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

func TestProjectsListPastTheEndPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":         []any{},
			"current_page": 2,
			"last_page":    1,
			"total":        7,
		})
	}))
	defer ts.Close()

	t.Run("table output", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := projectsListCmd(a)
		_ = cmd.Flags().Set("page", "2")

		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := buf.String()
		if strings.Contains(out, "No book projects found") {
			t.Errorf("expected NOT to show empty creation prompt, got: %q", out)
		}
		if !strings.Contains(out, "Page 2 is past the last page.") {
			t.Errorf("expected past-the-end notice, got: %q", out)
		}
		if !strings.Contains(out, "Page 2 of 1 (Total: 7)") {
			t.Errorf("expected pagination footer with total, got: %q", out)
		}
	})

	t.Run("json output untouched", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: true},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := projectsListCmd(a)
		_ = cmd.Flags().Set("page", "2")

		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if doc["current_page"] != float64(2) || doc["last_page"] != float64(1) || doc["total"] != float64(7) {
			t.Errorf("unexpected json payload: %s", buf.String())
		}
	})
}

func TestDownloadersListPastTheEndPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":         []any{},
			"current_page": 2,
			"last_page":    1,
			"total":        2,
		})
	}))
	defer ts.Close()

	t.Run("table output", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := downloadersListCmd(a)
		_ = cmd.Flags().Set("page", "2")

		if err := cmd.RunE(cmd, []string{"5"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := buf.String()
		if strings.Contains(out, "No downloaders found") {
			t.Errorf("expected NOT to show empty notice, got: %q", out)
		}
		if !strings.Contains(out, "Page 2 is past the last page.") {
			t.Errorf("expected past-the-end notice, got: %q", out)
		}
		if !strings.Contains(out, "Page 2 of 1 (Total: 2)") {
			t.Errorf("expected pagination footer with total, got: %q", out)
		}
	})

	t.Run("json output untouched", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: true},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := downloadersListCmd(a)
		_ = cmd.Flags().Set("page", "2")

		if err := cmd.RunE(cmd, []string{"5"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if doc["current_page"] != float64(2) || doc["last_page"] != float64(1) || doc["total"] != float64(2) {
			t.Errorf("unexpected json payload: %s", buf.String())
		}
	})
}

func TestLogsPastTheEndPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":         []any{},
			"current_page": 99,
			"last_page":    4,
			"total":        11,
		})
	}))
	defer ts.Close()

	t.Run("table output", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := logsCmd(a)
		_ = cmd.Flags().Set("page", "99")
		_ = cmd.Flags().Set("limit", "3")

		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := buf.String()
		if strings.Contains(out, "No activity logs recorded") {
			t.Errorf("expected NOT to show empty notice, got: %q", out)
		}
		if !strings.Contains(out, "Page 99 is past the last page.") {
			t.Errorf("expected past-the-end notice, got: %q", out)
		}
		if !strings.Contains(out, "Page 99 of 4 (total: 11)") {
			t.Errorf("expected pagination footer with total, got: %q", out)
		}
	})

	t.Run("json output untouched", func(t *testing.T) {
		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: true},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := logsCmd(a)
		_ = cmd.Flags().Set("page", "99")
		_ = cmd.Flags().Set("limit", "3")

		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if doc["current_page"] != float64(99) || doc["last_page"] != float64(4) || doc["total"] != float64(11) {
			t.Errorf("unexpected json payload: %s", buf.String())
		}
	})
}

func TestLogsEventFilterNoMatchesOnPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"occurred_at":  "2026-10-17T12:00:00Z",
					"type":         "signup",
					"reader_email": "reader@example.com",
					"book_title":   "Book One",
					"filename":     "book.epub",
				},
			},
			"current_page": 1,
			"last_page":    4,
			"total":        11,
		})
	}))
	defer ts.Close()

	var buf bytes.Buffer
	a := &app{
		printer:  &output.Printer{Out: &buf, JSON: false},
		settings: testSettings(t, ts.URL, "test-token"),
		apiCli:   client.New(ts.URL, "test-token"),
	}
	cmd := logsCmd(a)
	_ = cmd.Flags().Set("event", "no-such-type")
	_ = cmd.Flags().Set("limit", "3")

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "No activity logs recorded") {
		t.Errorf("expected NOT to show empty notice, got: %q", out)
	}
	if !strings.Contains(out, `No activity logs matched event "no-such-type" on this page.`) {
		t.Errorf("expected filter notice, got: %q", out)
	}
	if !strings.Contains(out, "Page 1 of 4 (total: 11)") {
		t.Errorf("expected pagination footer with total, got: %q", out)
	}
}

func TestTrulyEmptyCollectionsKeepStandardNotice(t *testing.T) {
	t.Run("projects list empty", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":         []any{},
				"current_page": 1,
				"last_page":    1,
				"total":        0,
			})
		}))
		defer ts.Close()

		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := projectsListCmd(a)
		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := buf.String(), "No book projects found. Create one with 'bookbeam projects create'.\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("downloaders list empty", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":         []any{},
				"current_page": 1,
				"last_page":    1,
				"total":        0,
			})
		}))
		defer ts.Close()

		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := downloadersListCmd(a)
		if err := cmd.RunE(cmd, []string{"5"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := buf.String(), "No downloaders found for this project.\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("logs empty", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":         []any{},
				"current_page": 1,
				"last_page":    1,
				"total":        0,
			})
		}))
		defer ts.Close()

		var buf bytes.Buffer
		a := &app{
			printer:  &output.Printer{Out: &buf, JSON: false},
			settings: testSettings(t, ts.URL, "test-token"),
			apiCli:   client.New(ts.URL, "test-token"),
		}
		cmd := logsCmd(a)
		if err := cmd.RunE(cmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := buf.String(), "No activity logs recorded.\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
