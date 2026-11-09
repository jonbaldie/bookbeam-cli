package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

func TestFormalAuthBackoffCases(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want int
		fail bool
	}{{"authorization_pending", 0, false}, {"slow_down", 5, false}, {"access_denied", 0, true}, {"expired_token", 0, true}, {"", 0, true}} {
		extra, err := pollBackoff(&client.APIError{ErrorType: tc.kind})
		if extra != tc.want || (err != nil) != tc.fail {
			t.Fatalf("%q got (%d,%v)", tc.kind, extra, err)
		}
	}
}

func TestFormalAuthFindingPollAfterExpiry(t *testing.T) {
	var requests []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, time.Now())
		if len(requests) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"authorization_pending"}`)
			return
		}
		io.WriteString(w, `{"access_token":"token"}`)
	}))
	defer server.Close()
	started := time.Now()
	token, err := pollDeviceToken(context.Background(), client.New(server.URL, ""), DeviceCodeResponse{ExpiresIn: 2, Interval: 1}, func(context.Context, time.Duration) error { time.Sleep(1100 * time.Millisecond); return nil })
	if err != nil || token != "token" || len(requests) != 2 {
		t.Fatalf("expected pending then successful poll, token=%q err=%v requests=%d", token, err, len(requests))
	}
	if requests[0].Sub(started) >= 2*time.Second || requests[1].Sub(started) < 2*time.Second {
		t.Fatalf("expected second poll after expiry, request times=%v", requests)
	}
}

func TestFormalAuthFindingEmptyTokenLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/code") {
			io.WriteString(w, `{"expires_in":60,"interval":1}`)
		} else {
			io.WriteString(w, `{}`)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	settings, err := config.Resolve(dir, func(string) string { return "" }, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := &app{settings: settings, apiCli: client.New(server.URL, ""), printer: &output.Printer{JSON: true, Out: &out}, sleep: func(context.Context, time.Duration) error { return nil }, openURL: func(string) {}}
	command := loginCmd(a)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	var result struct {
		LoggedIn bool `json:"logged_in"`
	}
	decodeErr := json.Unmarshal(out.Bytes(), &result)
	if err != nil || decodeErr != nil || !result.LoggedIn || settings.Token() != "" || strings.Contains(string(data), `"token"`) {
		t.Fatalf("empty-token finding not reproduced output=%q config=%q err=%v", out.String(), data, err)
	}
}

func TestFormalAuthReadinessAndClientSnapshot(t *testing.T) {
	failed := &app{}
	err := failed.load(func(string) string { return "" }, func() (string, error) { return "", errors.New("no home") }, config.Overrides{}, output.New(true, true))
	if err == nil || failed.settings != nil || failed.apiCli != nil || failed.printer != nil {
		t.Fatalf("failed resolution initialized app %+v error=%v", failed, err)
	}
	dir := t.TempDir()
	ready := &app{}
	getenv := func(key string) string {
		if key == config.ConfigDirEnv {
			return dir
		}
		return ""
	}
	if err := ready.load(getenv, func() (string, error) { return "", nil }, config.Overrides{}, output.New(true, true)); err != nil {
		t.Fatal(err)
	}
	if ready.settings == nil || ready.apiCli == nil || ready.printer == nil {
		t.Fatal("app not ready")
	}
	if err := ready.settings.StoreToken("new"); err != nil {
		t.Fatal(err)
	}
	if ready.apiCli.Token != "" || ready.settings.Token() != "new" {
		t.Fatalf("client snapshot finding not reproduced")
	}
}

func TestFormalAuthSaveFailureNoReport(t *testing.T) {
	dir := t.TempDir()
	settings, err := config.Resolve(dir, func(string) string { return "" }, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "config.json"), 0700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := &app{settings: settings, printer: &output.Printer{JSON: true, Out: &out}}
	command := loginCmd(a)
	command.SetArgs([]string{"--token", "new"})
	if err := command.Execute(); err == nil || out.Len() != 0 {
		t.Fatalf("save failure reported success, err=%v output=%q", err, out.String())
	}
}

func TestFormalReadinessResolutionBlocksCommand(t *testing.T) {
	ran := false
	root := newRootCmd(func(string) string { return "" }, func() (string, error) { return "", errors.New("no home") })
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.AddCommand(&cobra.Command{Use: "formal-probe", Annotations: map[string]string{publicCommand: "true"}, RunE: func(*cobra.Command, []string) error { ran = true; return nil }})
	root.SetArgs([]string{"formal-probe"})
	if err := root.Execute(); err == nil || ran {
		t.Fatalf("command ran through config failure, ran=%v err=%v", ran, err)
	}
}

func TestFormalAuthUnauthorizedIsTerminal(t *testing.T) {
	for _, kind := range []string{"authorization_pending", "slow_down"} {
		original := &client.APIError{StatusCode: http.StatusUnauthorized, ErrorType: kind}
		extra, err := pollBackoff(original)
		if err != original || extra != 0 {
			t.Fatalf("401 %q was retried: %d %v", kind, extra, err)
		}
	}
}

func TestFormalAuthCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait returned %v", err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; io.WriteString(w, `{"access_token":"new"}`) }))
	defer server.Close()
	token, err := pollDeviceToken(ctx, client.New(server.URL, ""), DeviceCodeResponse{ExpiresIn: 60, Interval: 1}, waitContext)
	if !errors.Is(err, context.Canceled) || token != "" || requests != 0 {
		t.Fatalf("cancelled poll token=%q err=%v requests=%d", token, err, requests)
	}
}

type formalCancelBody struct {
	io.Reader
	cancel context.CancelFunc
}

func (b *formalCancelBody) Close() error { b.cancel(); return nil }

type formalRoundTrip func(*http.Request) (*http.Response, error)

func (f formalRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFormalAuthCancellationBeforeSave(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	settings, err := config.Resolve(t.TempDir(), func(string) string { return "" }, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	api := client.New("https://example.test", "")
	api.HTTPClient.Transport = formalRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"expires_in":60,"interval":1}`
		var stream io.ReadCloser = io.NopCloser(strings.NewReader(body))
		if strings.HasSuffix(r.URL.Path, "/token") {
			stream = &formalCancelBody{Reader: strings.NewReader(`{"access_token":"new"}`), cancel: cancel}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: stream, Request: r}, nil
	})
	var out bytes.Buffer
	a := &app{settings: settings, apiCli: api, printer: &output.Printer{JSON: true, Out: &out}, sleep: func(context.Context, time.Duration) error { return nil }, openURL: func(string) {}}
	command := loginCmd(a)
	err = command.ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) || settings.Token() != "" || out.Len() != 0 {
		t.Fatalf("cancel before save err=%v token=%q out=%q", err, settings.Token(), out.String())
	}
}

func TestFormalAuthenticationGate(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, token := range []string{"", "token"} {
			dir := t.TempDir()
			ran := false
			root := newRootCmd(func(k string) string {
				if k == config.ConfigDirEnv {
					return dir
				}
				if k == config.TokenEnv {
					return token
				}
				return ""
			}, func() (string, error) { return "", nil })
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			probe := &cobra.Command{Use: "formal-gate", RunE: func(*cobra.Command, []string) error { ran = true; return nil }}
			if public {
				probe.Annotations = map[string]string{publicCommand: "true"}
			}
			root.AddCommand(probe)
			root.SetArgs([]string{"formal-gate"})
			err := root.Execute()
			want := public || token != ""
			if ran != want || (err == nil) != want {
				t.Fatalf("public=%v token=%q ran=%v err=%v", public, token, ran, err)
			}
		}
	}
}
