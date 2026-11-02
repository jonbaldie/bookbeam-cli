package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

type slowUploadTransport struct{ status int }

func (s slowUploadTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	defer r.Body.Close()
	buf := make([]byte, 16384)
	for {
		_, err := r.Body.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		time.Sleep(110 * time.Millisecond)
	}
	return &http.Response{StatusCode: s.status, Body: io.NopCloser(strings.NewReader(`{"data":{"id":56,"filename":"sample.epub"}}`)), Header: make(http.Header)}, nil
}

func TestUploadProgressAndOutputModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.epub")
	if err := os.WriteFile(path, make([]byte, 192*1024), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		quiet, json bool
		status      int
	}{
		{"normal", false, false, 200},
		{"quiet", true, false, 200},
		{"json", false, true, 200},
		{"failure", false, false, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			api := client.New("http://example.test", "test")
			api.HTTPClient = &http.Client{Transport: slowUploadTransport{status: tc.status}}
			a := &app{apiCli: api, printer: &output.Printer{Out: &out, Quiet: tc.quiet, JSON: tc.json}}
			err := runCommand(filesUploadCmd(a), []string{"10", path})
			got := out.String()
			if tc.status == 500 {
				if err == nil || strings.Contains(got, "successfully") {
					t.Fatalf("failure err=%v output=%q", err, got)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !tc.quiet && !tc.json && !strings.Contains(got, "Transferred ") {
				t.Errorf("missing intermediate file-byte progress: %q", got)
			}
			if tc.quiet && got != "" {
				t.Errorf("quiet output: %q", got)
			}
			if tc.json {
				var doc struct {
					Data struct {
						ID int `json:"id"`
					} `json:"data"`
				}
				if err := json.Unmarshal(out.Bytes(), &doc); err != nil || doc.Data.ID != 56 {
					t.Errorf("invalid result %q: %v", got, err)
				}
			}
		})
	}
}
