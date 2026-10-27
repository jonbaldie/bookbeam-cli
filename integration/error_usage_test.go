package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorsSuppressUsage(t *testing.T) {
	for _, status := range []int{401, 403, 404, 422, 500} {
		for _, format := range []string{"table", "json"} {
			t.Run(fmt.Sprintf("%d/%s", status, format), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"Request rejected."}`)
				}))
				defer server.Close()
				args := []string{"--host", server.URL, "--token", "fixture-token", "projects", "get", "9999"}
				if format == "json" {
					args = append(args, "--json")
				}
				out, err := runCLIWithHome(t, t.TempDir(), args...)
				want := fmt.Sprintf("Error: API error (%d): Request rejected.\n", status)
				if status == 401 {
					want = "Error: Not authenticated. Run 'bookbeam auth login' to authenticate.\n"
				}
				if err == nil || out != want {
					t.Fatalf("expected a failed command with only one error line, got err=%v output=%q", err, out)
				}
			})
		}
	}
}

func TestUsageErrorsStillShowUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing argument", []string{"projects", "get"}, "Error: accepts 1 arg(s), received 0\n"},
		{"extra argument", []string{"projects", "get", "1", "2"}, "Error: accepts 1 arg(s), received 2\n"},
		{"unknown flag", []string{"projects", "get", "1", "--unknown"}, "Error: unknown flag: --unknown\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCLIWithHome(t, t.TempDir(), tc.args...)
			if err == nil || !strings.Contains(out, "Usage:\n") || !strings.Contains(out, tc.want) || strings.Count(out, "Error:") != 1 {
				t.Fatalf("expected usage and one error, got err=%v output=%q", err, out)
			}
		})
	}
}
