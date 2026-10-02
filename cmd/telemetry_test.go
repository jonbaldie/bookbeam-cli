package cmd

import (
	"strings"
	"testing"
)

// logsMiddlePage is page 2 of 6 from GET /api/v1/logs (issue #54).
const logsMiddlePage = `{"current_page":2,"data":[` +
	`{"id":11,"type":"download","occurred_at":"2026-09-15T08:00:01.000000Z",` +
	`"reader_email":"reader@example.com","book_title":"Reader Magnet Playbook",` +
	`"filename":"playbook.pdf","signup_link_slug":"iNS2rgt9RS"}],` +
	`"last_page":6,"per_page":10,"total":58}`

func TestLogsPageQuery(t *testing.T) {
	cases := []struct {
		name, page, query string
	}{
		{"default page", "", "page=1&per_page=50"},
		{"page two", "2", "page=2&per_page=50"},
		{"zero page omitted", "0", "per_page=50"},
		{"negative page omitted", "-1", "per_page=50"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, seen := ntdServer(t, 200, `{"data":[]}`)
			a, _ := ntdApp(ts.URL, false)
			cmd := logsCmd(a)
			if tc.page != "" {
				if err := cmd.Flags().Set("page", tc.page); err != nil {
					t.Fatal(err)
				}
			}
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			req := ntdOnly(t, seen)
			if req.Path != "/api/v1/logs" || req.Query != tc.query {
				t.Errorf("unexpected request %+v, want query %q", req, tc.query)
			}
		})
	}
}

func TestLogsPaginationFooter(t *testing.T) {
	ts, _ := ntdServer(t, 200, logsMiddlePage)
	a, buf := ntdApp(ts.URL, false)
	cmd := logsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	want := "TIME                          TYPE       READER               BOOK                     FILE\n" +
		"2026-09-15T08:00:01.000000Z   download   reader@example.com   Reader Magnet Playbook   playbook.pdf\n" +
		"\nPage 2 of 6 (Total: 58)\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// A single page needs no footer: ntdLogsPage has last_page 1.
func TestLogsSinglePageHasNoFooter(t *testing.T) {
	ts, _ := ntdServer(t, 200, ntdLogsPage)
	a, buf := ntdApp(ts.URL, false)
	cmd := logsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); len(got) == 0 || strings.Contains(got, "Page ") {
		t.Errorf("expected table without footer, got %q", got)
	}
}

func TestLogsJSONKeepsPagination(t *testing.T) {
	ts, _ := ntdServer(t, 200, logsMiddlePage)
	a, buf := ntdApp(ts.URL, true)
	cmd := logsCmd(a)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"current_page": 2`, `"last_page": 6`, `"total": 58`, `"per_page": 10`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %s in %q", want, buf.String())
		}
	}
}

// Two pages is the smallest result that needs the footer.
func TestLogsFooterBoundary(t *testing.T) {
	if got := logsFooter(ActivityLogPage{CurrentPage: 1, LastPage: 2, Total: 12}); got != "\nPage 1 of 2 (Total: 12)" {
		t.Errorf("got %q", got)
	}
	if got := logsFooter(ActivityLogPage{CurrentPage: 1, LastPage: 1, Total: 3}); got != "" {
		t.Errorf("got %q", got)
	}
}
