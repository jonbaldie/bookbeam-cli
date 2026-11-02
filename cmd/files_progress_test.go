package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

type timedUploadOutput struct {
	bytes.Buffer
	updates []time.Time
}

func (o *timedUploadOutput) Write(p []byte) (int, error) {
	if strings.HasPrefix(string(p), "Transferred ") {
		o.updates = append(o.updates, time.Now())
	}
	return o.Buffer.Write(p)
}

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
			var out timedUploadOutput
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
			if !tc.quiet && !tc.json {
				var transferred int64
				start := strings.Index(got, "Transferred ")
				if start < 0 || func() bool {
					_, err := fmt.Sscanf(got[start:], "Transferred %d file bytes to the transport", &transferred)
					return err != nil || transferred <= 0 || transferred > 192*1024
				}() {
					t.Errorf("invalid file-byte progress: %q", got)
				}
				for i := 1; i < len(out.updates); i++ {
					if out.updates[i].Sub(out.updates[i-1]) < time.Second {
						t.Errorf("progress updates less than one second apart: %v", out.updates)
					}
				}
				if tc.status == 200 && start > strings.Index(got, "successfully") {
					t.Errorf("progress after success: %q", got)
				}
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

func TestUploadEmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"data":{"id":56,"filename":"empty.epub"}}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "empty.epub")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := &app{apiCli: client.New(server.URL, "test"), printer: &output.Printer{Out: &out}}
	if err := runCommand(filesUploadCmd(a), []string{"10", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Uploaded file #56 (empty.epub) successfully.") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
