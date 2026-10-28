package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

type exportTransport struct{}

func (exportTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("Email\nx@example.com\n"))}, nil
}

type exportWriter struct {
	n   int
	err error
}

func (w exportWriter) Write([]byte) (int, error) { return w.n, w.err }

func TestDownloadersExportStdoutOutput(t *testing.T) {
	for _, tc := range []struct {
		name        string
		json, quiet bool
		stderr      string
	}{
		{"normal", false, false, "✓ Exported 20 bytes to stdout\n"},
		{"quiet", false, true, ""},
		{"json", true, false, "✓ Exported 20 bytes to stdout\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := client.New("http://example.com", "token")
			api.HTTPClient = &http.Client{Transport: exportTransport{}}
			var stdout, stderr bytes.Buffer
			a := &app{apiCli: api, printer: &output.Printer{Out: &stdout, JSON: tc.json, Quiet: tc.quiet}}
			cmd := downloadersExportCmd(a)
			cmd.SetErr(&stderr)
			_ = cmd.Flags().Set("output", "-")
			if err := runCommand(cmd, []string{"12"}); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); got != "Email\nx@example.com\n" {
				t.Errorf("stdout = %q", got)
			}
			if got := stderr.String(); got != tc.stderr {
				t.Errorf("stderr = %q, want %q", got, tc.stderr)
			}
		})
	}
}

func TestDownloadersExportStdoutWriter(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	failure := errors.New("write failed")
	for _, tc := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{"error", exportWriter{0, failure}, failure},
		{"short write", exportWriter{1, nil}, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := client.New("http://example.com", "token")
			api.HTTPClient = &http.Client{Transport: exportTransport{}}
			a := &app{apiCli: api, printer: &output.Printer{Out: tc.writer}}
			cmd := downloadersExportCmd(a)
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)
			_ = cmd.Flags().Set("output", "-")
			err := runCommand(cmd, []string{"12"})
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected confirmation on failure: %q", stderr.String())
			}
		})
	}
}
