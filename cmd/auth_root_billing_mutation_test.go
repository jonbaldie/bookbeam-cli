package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

// arbIsolate points HOME at a temp dir and clears BookBeam env overrides.
func arbIsolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BOOKBEAM_HOST", "")
	t.Setenv("BOOKBEAM_TOKEN", "")
	return home
}

// arbBreakHome makes config saves fail by placing a file where the config dir must go.
func arbBreakHome(t *testing.T) {
	t.Helper()
	home := arbIsolate(t)
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
}

// arbAssertWrapped fails unless err keeps its cause reachable via errors.Unwrap.
func arbAssertWrapped(t *testing.T, err error) {
	t.Helper()
	if errors.Unwrap(err) == nil {
		t.Errorf("error %q does not wrap its cause", err)
	}
}

func arbSavedConfig(t *testing.T, home string) config.Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".config", "bookbeam", "config.json"))
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing saved config: %v", err)
	}
	return cfg
}

type arbHarness struct {
	a       *app
	out     *bytes.Buffer
	opened  []string
	sleeps  []time.Duration
	baseURL string
}

func arbNewHarness(t *testing.T, handler http.HandlerFunc, jsonOut bool) *arbHarness {
	t.Helper()
	h := &arbHarness{out: &bytes.Buffer{}}
	base := "http://127.0.0.1:1"
	if handler != nil {
		ts := httptest.NewServer(handler)
		t.Cleanup(ts.Close)
		base = ts.URL
	}
	h.baseURL = base
	h.a = &app{
		cfg:     &config.Config{Host: base, Token: "tok"},
		apiCli:  client.New(base, "tok"),
		printer: &output.Printer{Out: h.out, JSON: jsonOut},
		openURL: func(url string) { h.opened = append(h.opened, url) },
		sleep:   func(d time.Duration) { h.sleeps = append(h.sleeps, d) },
	}
	return h
}

func arbRun(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	return cmd.RunE(cmd, args)
}

func arbWriteJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestARBCommandMetadata(t *testing.T) {
	a := &app{}
	cases := []struct {
		cmd   *cobra.Command
		use   string
		short string
	}{
		{authCmd(a), "auth", "Manage authentication and tokens"},
		{loginCmd(a), "login", "Authenticate with BookBeam via browser or direct token"},
		{logoutCmd(a), "logout", "Log out of BookBeam and remove saved credentials"},
		{whoamiCmd(a), "whoami", "Display the currently authenticated user and team"},
		{billingCmd(a), "billing", "Check team account billing and subscription entitlement"},
		{billingStatusCmd(a), "status", "Show current team subscription status and active plan"},
		{billingCheckoutCmd(a), "checkout", "Open or display the checkout link for the active plan"},
		{completionCmd(), "completion [bash|zsh|fish|powershell]", "Generate shell completion script for your terminal"},
		{NewRootCmd(), "bookbeam", "Official command-line interface for BookBeam (bookbeam.app)"},
	}
	for _, tc := range cases {
		if tc.cmd.Use != tc.use || tc.cmd.Short != tc.short {
			t.Errorf("got Use=%q Short=%q, want Use=%q Short=%q", tc.cmd.Use, tc.cmd.Short, tc.use, tc.short)
		}
	}
	if !strings.Contains(completionCmd().Long, "source <(bookbeam completion bash)") {
		t.Error("completion help lost its bash instructions")
	}
	if !strings.Contains(NewRootCmd().Long, "manage book projects, files") {
		t.Error("root long description missing")
	}
	flag := loginCmd(a).Flags().Lookup("token")
	if flag == nil || flag.DefValue != "" || flag.Usage != "Authenticate directly with an API personal access token" {
		t.Errorf("unexpected login --token flag: %+v", flag)
	}
}

func arbSubcommandNames(cmd *cobra.Command) []string {
	var names []string
	for _, c := range cmd.Commands() {
		names = append(names, c.Name())
	}
	return names
}

func TestARBCommandTree(t *testing.T) {
	a := &app{}
	if got, want := arbSubcommandNames(authCmd(a)), []string{"login", "logout", "whoami"}; !reflect.DeepEqual(got, want) {
		t.Errorf("auth subcommands = %v, want %v", got, want)
	}
	if got, want := arbSubcommandNames(billingCmd(a)), []string{"checkout", "status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("billing subcommands = %v, want %v", got, want)
	}
	want := []string{"auth", "billing", "completion", "downloaders", "files", "links", "logs", "metrics", "newsletter", "projects", "whoami"}
	if got := arbSubcommandNames(NewRootCmd()); !reflect.DeepEqual(got, want) {
		t.Errorf("root subcommands = %v, want %v", got, want)
	}
}

func TestARBRootFlags(t *testing.T) {
	root := NewRootCmd()
	pf := root.PersistentFlags()
	for name, usage := range map[string]string{
		"host":  "BookBeam API host (default https://bookbeam.app)",
		"token": "BookBeam API personal access token",
		"json":  "Output results as JSON",
		"quiet": "Suppress informational messages",
	} {
		f := pf.Lookup(name)
		if f == nil || f.Usage != usage {
			t.Errorf("flag %s: got %+v", name, f)
		}
	}
	if pf.Lookup("quiet").Shorthand != "q" {
		t.Error("quiet should have -q shorthand")
	}
	if !root.SilenceErrors {
		t.Error("root should silence errors so main prints them once")
	}
}

func TestARBRootAppliesFlagsToWhoami(t *testing.T) {
	arbIsolate(t)
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		arbWriteJSON(w, 200, `{"id":1,"name":"Ann","email":"ann@example.com"}`)
	}))
	defer ts.Close()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--host", ts.URL, "--token", "flag-token", "--json", "whoami"})
	// Printer writes to os.Stdout; capture it.
	stdout := arbCaptureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if gotAuth != "Bearer flag-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(stdout, `"email": "ann@example.com"`) {
		t.Errorf("expected JSON output, got %q", stdout)
	}
}

func TestARBRootQuietSuppressesInfo(t *testing.T) {
	home := arbIsolate(t)
	root := NewRootCmd()
	root.SetArgs([]string{"-q", "auth", "logout"})
	stdout := arbCaptureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if stdout != "" {
		t.Errorf("quiet should suppress output, got %q", stdout)
	}
	if cfg := arbSavedConfig(t, home); cfg.Host != "https://bookbeam.app" || cfg.Token != "" {
		t.Errorf("unexpected saved config %+v", cfg)
	}
}

func TestARBRootUsesConfigWithoutFlags(t *testing.T) {
	home := arbIsolate(t)
	root := NewRootCmd()
	root.SetArgs([]string{"auth", "logout"})
	stdout := arbCaptureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if stdout != "✓ Logged out successfully.\n" {
		t.Errorf("got %q", stdout)
	}
	if cfg := arbSavedConfig(t, home); cfg.Host != "https://bookbeam.app" {
		t.Errorf("host should default, got %+v", cfg)
	}
}

func TestARBRootReportsConfigLoadFailure(t *testing.T) {
	home := arbIsolate(t)
	dir := filepath.Join(home, ".config", "bookbeam")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"auth", "logout"})
	err := root.Execute()
	if err == nil || !strings.HasPrefix(err.Error(), "failed to load configuration: failed to parse config file") {
		t.Fatalf("got %v", err)
	}
}

func arbCaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	defer func() { os.Stdout = old }()
	fn()
	os.Stdout = old
	_ = w.Close()
	return <-done
}

func TestARBLoad(t *testing.T) {
	printer := &output.Printer{}
	t.Run("overrides", func(t *testing.T) {
		a := &app{}
		err := a.load(func(path string) (*config.Config, error) {
			if path != "" {
				t.Errorf("load path = %q, want default", path)
			}
			return &config.Config{Host: "https://file.example.com", Token: "file-token"}, nil
		}, "https://flag.example.com", "flag-token", printer)
		if err != nil {
			t.Fatal(err)
		}
		if a.cfg.Host != "https://flag.example.com" || a.cfg.Token != "flag-token" {
			t.Errorf("cfg = %+v", a.cfg)
		}
		if a.apiCli.BaseURL != "https://flag.example.com" || a.apiCli.Token != "flag-token" {
			t.Errorf("client = %+v", a.apiCli)
		}
		if a.printer != printer {
			t.Error("printer not set")
		}
	})
	t.Run("no overrides", func(t *testing.T) {
		a := &app{}
		err := a.load(func(string) (*config.Config, error) {
			return &config.Config{Host: "https://file.example.com", Token: "file-token"}, nil
		}, "", "", printer)
		if err != nil {
			t.Fatal(err)
		}
		if a.apiCli.BaseURL != "https://file.example.com" || a.apiCli.Token != "file-token" {
			t.Errorf("client = %+v", a.apiCli)
		}
	})
	t.Run("error", func(t *testing.T) {
		a := &app{}
		cause := errors.New("boom")
		err := a.load(func(string) (*config.Config, error) {
			return nil, cause
		}, "", "", printer)
		if !errors.Is(err, cause) {
			t.Errorf("load error should wrap its cause")
		}
		if err == nil || err.Error() != "failed to load configuration: boom" {
			t.Fatalf("got %v", err)
		}
		if a.cfg != nil || a.apiCli != nil || a.printer != nil {
			t.Error("app should be untouched on error")
		}
	})
}

func TestARBExecuteReturnsError(t *testing.T) {
	arbIsolate(t)
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"bookbeam", "nonexistent"}
	err := Execute()
	if err == nil || !strings.Contains(err.Error(), `unknown command "nonexistent" for "bookbeam"`) {
		t.Fatalf("got %v", err)
	}
}

func TestARBConfirm(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"  YES \n", true},
		{"Y", true},
		{"n\n", false},
		{"yep\n", false},
		{"\n", false},
		{"", false},
	}
	for _, tc := range cases {
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader(tc.input))
		if got := confirm(cmd, "Delete it?"); got != tc.want {
			t.Errorf("confirm(%q) = %v, want %v", tc.input, got, tc.want)
		}
		if out.String() != "Delete it? (y/N): " {
			t.Errorf("prompt = %q", out.String())
		}
	}
}

func TestARBCompletionScripts(t *testing.T) {
	arbIsolate(t)
	cases := map[string][]string{
		"bash":       {"# bash completion for bookbeam"},
		"zsh":        {"#compdef bookbeam"},
		"fish":       {"# fish completion for bookbeam", "__complete "},
		"powershell": {"# powershell completion for bookbeam", "__complete "},
	}
	for shell, markers := range cases {
		root := NewRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"completion", shell})
		if err := root.Execute(); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, m := range markers {
			if !strings.Contains(out.String(), m) {
				t.Errorf("%s output missing %q", shell, m)
			}
		}
		if (shell == "fish" || shell == "powershell") && strings.Contains(out.String(), "__completeNoDesc") {
			t.Errorf("%s completion should include descriptions", shell)
		}
	}
}

func TestARBCompletionRejectsBadArgs(t *testing.T) {
	arbIsolate(t)
	for args, want := range map[string]string{
		"tcsh": `invalid argument "tcsh" for "bookbeam completion"`,
		"":     "accepts 1 arg(s), received 0",
	} {
		root := NewRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		argv := []string{"completion"}
		if args != "" {
			argv = append(argv, args)
		}
		root.SetArgs(argv)
		err := root.Execute()
		if err == nil || err.Error() != want {
			t.Errorf("args %q: got %v", args, err)
		}
		if !strings.Contains(out.String(), "Usage:\n  bookbeam completion [bash|zsh|fish|powershell]\n") {
			t.Errorf("usage line should hide [flags], got %q", out.String())
		}
	}
}

func TestARBCompletionUnsupportedShellDirect(t *testing.T) {
	cmd := completionCmd()
	err := cmd.RunE(cmd, []string{"tcsh"})
	if err == nil || err.Error() != "unsupported shell type tcsh" {
		t.Fatalf("got %v", err)
	}
}

func TestARBCompletionSuggestsShells(t *testing.T) {
	arbIsolate(t)
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"__complete", "completion", ""})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "bash\nzsh\nfish\npowershell\n") {
		t.Errorf("got %q", out.String())
	}
}

func TestARBBrowserCommand(t *testing.T) {
	cases := map[string][]string{
		"darwin":  {"open", "https://x.example.com"},
		"windows": {"rundll32", "url.dll,FileProtocolHandler", "https://x.example.com"},
		"linux":   {"xdg-open", "https://x.example.com"},
		"freebsd": {"xdg-open", "https://x.example.com"},
	}
	for goos, want := range cases {
		if got := browserCommand(goos, "https://x.example.com").Args; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", goos, got, want)
		}
	}
}

func TestARBDirectTokenLogin(t *testing.T) {
	home := arbIsolate(t)
	h := arbNewHarness(t, nil, false)
	cmd := loginCmd(h.a)
	_ = cmd.Flags().Set("token", "direct-123")
	if err := arbRun(t, cmd); err != nil {
		t.Fatal(err)
	}
	if h.out.String() != "✓ Authentication token saved successfully.\n" {
		t.Errorf("got %q", h.out.String())
	}
	if cfg := arbSavedConfig(t, home); cfg.Token != "direct-123" {
		t.Errorf("saved %+v", cfg)
	}
	if len(h.opened) != 0 {
		t.Error("direct token login must not open a browser")
	}
}

func TestARBDirectTokenLoginSaveFailure(t *testing.T) {
	arbBreakHome(t)
	h := arbNewHarness(t, nil, false)
	cmd := loginCmd(h.a)
	_ = cmd.Flags().Set("token", "direct-123")
	err := arbRun(t, cmd)
	if err == nil || !strings.HasPrefix(err.Error(), "failed to save token to config: ") {
		t.Fatalf("got %v", err)
	}
	arbAssertWrapped(t, err)
	if h.out.Len() != 0 {
		t.Errorf("no success message expected, got %q", h.out.String())
	}
}

type arbDeviceServer struct {
	codeBody     string
	codeStatus   int
	tokenReplies []struct {
		status int
		body   string
	}
	codePayloads  []map[string]string
	tokenPayloads []map[string]string
}

func (s *arbDeviceServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		switch r.URL.Path {
		case "/api/v1/oauth/device/code":
			s.codePayloads = append(s.codePayloads, payload)
			arbWriteJSON(w, s.codeStatus, s.codeBody)
		case "/api/v1/oauth/device/token":
			s.tokenPayloads = append(s.tokenPayloads, payload)
			if len(s.tokenReplies) == 0 {
				t.Errorf("unexpected extra token poll")
				arbWriteJSON(w, 500, `{}`)
				return
			}
			reply := s.tokenReplies[0]
			s.tokenReplies = s.tokenReplies[1:]
			arbWriteJSON(w, reply.status, reply.body)
		default:
			http.NotFound(w, r)
		}
	}
}

func (s *arbDeviceServer) reply(status int, body string) {
	s.tokenReplies = append(s.tokenReplies, struct {
		status int
		body   string
	}{status, body})
}

const arbDeviceCode = `{"device_code":"dev-1","user_code":"ABCD-EFGH","verification_uri":"https://v.example.com","verification_uri_complete":"https://v.example.com/?code=ABCD-EFGH","expires_in":600,"interval":3}`

func TestARBDeviceLoginSuccessAfterPendingAndSlowDown(t *testing.T) {
	home := arbIsolate(t)
	s := &arbDeviceServer{codeStatus: 200, codeBody: arbDeviceCode}
	s.reply(400, `{"error":"authorization_pending"}`)
	s.reply(400, `{"error":"slow_down"}`)
	s.reply(200, `{"access_token":"device-token","token_type":"Bearer","token_id":7,"team_id":3}`)
	h := arbNewHarness(t, s.handler(t), false)

	cmd := loginCmd(h.a)
	if err := arbRun(t, cmd); err != nil {
		t.Fatal(err)
	}

	want := "Initiating device login with " + h.baseURL + "...\n" +
		"\n" +
		"! Your one-time verification code is: ABCD-EFGH\n" +
		"- Open verification page: https://v.example.com/?code=ABCD-EFGH\n" +
		"\n" +
		"Waiting for authorization in browser...\n" +
		"\n" +
		"✓ Successfully authenticated! Logged in to BookBeam.\n"
	if h.out.String() != want {
		t.Errorf("output:\n%q\nwant:\n%q", h.out.String(), want)
	}
	if !reflect.DeepEqual(h.opened, []string{"https://v.example.com/?code=ABCD-EFGH"}) {
		t.Errorf("opened %v", h.opened)
	}
	if want := []time.Duration{3 * time.Second, 3 * time.Second, 8 * time.Second}; !reflect.DeepEqual(h.sleeps, want) {
		t.Errorf("sleeps %v, want %v", h.sleeps, want)
	}
	if want := []map[string]string{{"client_id": "bookbeam-cli"}}; !reflect.DeepEqual(s.codePayloads, want) {
		t.Errorf("code payloads %v", s.codePayloads)
	}
	wantToken := map[string]string{
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		"device_code": "dev-1",
		"client_id":   "bookbeam-cli",
	}
	if len(s.tokenPayloads) != 3 || !reflect.DeepEqual(s.tokenPayloads[2], wantToken) {
		t.Errorf("token payloads %v", s.tokenPayloads)
	}
	if h.a.cfg.Token != "device-token" {
		t.Errorf("cfg token %q", h.a.cfg.Token)
	}
	if cfg := arbSavedConfig(t, home); cfg.Token != "device-token" {
		t.Errorf("saved %+v", cfg)
	}
}

func TestARBDeviceLoginDefaultsIntervalToFiveSeconds(t *testing.T) {
	arbIsolate(t)
	s := &arbDeviceServer{codeStatus: 200, codeBody: `{"device_code":"d","user_code":"U","verification_uri_complete":"https://v.example.com","expires_in":60,"interval":0}`}
	s.reply(200, `{"access_token":"t"}`)
	h := arbNewHarness(t, s.handler(t), false)
	if err := arbRun(t, loginCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{5 * time.Second}; !reflect.DeepEqual(h.sleeps, want) {
		t.Errorf("sleeps %v", h.sleeps)
	}
}

func TestARBDeviceLoginIntervalOfOneIsKept(t *testing.T) {
	arbIsolate(t)
	s := &arbDeviceServer{codeStatus: 200, codeBody: `{"device_code":"d","user_code":"U","verification_uri_complete":"https://v.example.com","expires_in":60,"interval":1}`}
	s.reply(200, `{"access_token":"t"}`)
	h := arbNewHarness(t, s.handler(t), false)
	if err := arbRun(t, loginCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{time.Second}; !reflect.DeepEqual(h.sleeps, want) {
		t.Errorf("sleeps %v", h.sleeps)
	}
}

func TestARBDeviceLoginTimesOut(t *testing.T) {
	arbIsolate(t)
	s := &arbDeviceServer{codeStatus: 200, codeBody: `{"device_code":"d","user_code":"U","verification_uri_complete":"https://v.example.com","expires_in":0,"interval":2}`}
	h := arbNewHarness(t, s.handler(t), false)
	err := arbRun(t, loginCmd(h.a))
	if err == nil || err.Error() != "device authorization timed out; please run 'bookbeam auth login' again" {
		t.Fatalf("got %v", err)
	}
	if len(h.sleeps) != 0 || len(s.tokenPayloads) != 0 {
		t.Errorf("expired code should not poll: sleeps=%v polls=%d", h.sleeps, len(s.tokenPayloads))
	}
}

func TestARBPollDeviceTokenStopsAtDeadline(t *testing.T) {
	var polls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls++
		arbWriteJSON(w, 400, `{"error":"authorization_pending"}`)
	}))
	defer ts.Close()
	sleep := func(time.Duration) { time.Sleep(600 * time.Millisecond) }
	_, err := pollDeviceToken(client.New(ts.URL, ""), DeviceCodeResponse{DeviceCode: "d", ExpiresIn: 1, Interval: 1}, sleep)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("got %v", err)
	}
	if polls != 2 {
		t.Errorf("polls = %d, want 2", polls)
	}
}

func TestARBDeviceLoginFailures(t *testing.T) {
	cases := []struct {
		name       string
		codeStatus int
		codeBody   string
		token      []string
		tokenCode  int
		wantPrefix string
		wantExact  string
	}{
		{name: "code request fails", codeStatus: 500, codeBody: `{"message":"down"}`, wantExact: "failed to request device authorization code: API error (500): down"},
		{name: "code response invalid", codeStatus: 200, codeBody: `not json`, wantPrefix: "invalid server response: "},
		{name: "denied", codeStatus: 200, codeBody: arbDeviceCode, token: []string{`{"error":"access_denied","error_description":"User said no"}`}, tokenCode: 400, wantExact: "authorization failed: User said no"},
		{name: "expired", codeStatus: 200, codeBody: arbDeviceCode, token: []string{`{"error":"expired_token"}`}, tokenCode: 400, wantExact: "authorization failed: "},
		{name: "token response invalid", codeStatus: 200, codeBody: arbDeviceCode, token: []string{`not json`}, tokenCode: 200, wantPrefix: "failed to parse token response: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			arbIsolate(t)
			s := &arbDeviceServer{codeStatus: tc.codeStatus, codeBody: tc.codeBody}
			for _, body := range tc.token {
				s.reply(tc.tokenCode, body)
			}
			h := arbNewHarness(t, s.handler(t), false)
			err := arbRun(t, loginCmd(h.a))
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.wantExact != "" && err.Error() != tc.wantExact {
				t.Errorf("got %q, want %q", err.Error(), tc.wantExact)
			}
			if tc.wantPrefix != "" && !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Errorf("got %q, want prefix %q", err.Error(), tc.wantPrefix)
			}
			if !strings.HasPrefix(tc.wantExact, "authorization failed") {
				arbAssertWrapped(t, err)
			}
			if h.a.cfg.Token != "tok" {
				t.Errorf("token should be unchanged, got %q", h.a.cfg.Token)
			}
		})
	}
}

func TestARBDeviceLoginSaveFailure(t *testing.T) {
	arbBreakHome(t)
	s := &arbDeviceServer{codeStatus: 200, codeBody: arbDeviceCode}
	s.reply(200, `{"access_token":"device-token"}`)
	h := arbNewHarness(t, s.handler(t), false)
	err := arbRun(t, loginCmd(h.a))
	if err == nil || !strings.HasPrefix(err.Error(), "failed to save token to config: ") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(h.out.String(), "Successfully authenticated") {
		t.Error("success message printed despite save failure")
	}
}

func TestARBPollBackoff(t *testing.T) {
	plain := errors.New("network down")
	if extra, err := pollBackoff(plain); extra != 0 || err != plain {
		t.Errorf("non-API error: %d, %v", extra, err)
	}
	if extra, err := pollBackoff(&client.APIError{ErrorType: "authorization_pending"}); extra != 0 || err != nil {
		t.Errorf("pending: %d, %v", extra, err)
	}
	if extra, err := pollBackoff(&client.APIError{ErrorType: "slow_down"}); extra != 5 || err != nil {
		t.Errorf("slow_down: %d, %v", extra, err)
	}
	if extra, err := pollBackoff(&client.APIError{ErrorType: "access_denied", ErrorDescription: "nope"}); extra != 0 || err == nil || err.Error() != "authorization failed: nope" {
		t.Errorf("denied: %d, %v", extra, err)
	}
}

func TestARBPollDeviceTokenNetworkError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := ts.URL
	ts.Close()
	token, err := pollDeviceToken(client.New(url, ""), DeviceCodeResponse{ExpiresIn: 60, Interval: 1}, func(time.Duration) {})
	if err == nil || token != "" {
		t.Fatalf("got %q, %v", token, err)
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) || strings.HasPrefix(err.Error(), "authorization failed") {
		t.Errorf("network error should pass through unchanged, got %v", err)
	}
}

func TestARBParseDeviceToken(t *testing.T) {
	token, err := parseDeviceToken([]byte(`{"access_token":"abc"}`))
	if err != nil || token != "abc" {
		t.Errorf("got %q, %v", token, err)
	}
	token, err = parseDeviceToken([]byte(`nope`))
	if err == nil || token != "" {
		t.Fatalf("got %q, %v", token, err)
	}
	arbAssertWrapped(t, err)
}

func TestARBLogout(t *testing.T) {
	home := arbIsolate(t)
	h := arbNewHarness(t, nil, false)
	h.a.cfg = &config.Config{Host: "https://h.example.com", Token: "secret"}
	if err := arbRun(t, logoutCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if h.out.String() != "✓ Logged out successfully.\n" {
		t.Errorf("got %q", h.out.String())
	}
	if cfg := arbSavedConfig(t, home); cfg.Token != "" || cfg.Host != "https://h.example.com" {
		t.Errorf("saved %+v", cfg)
	}
	if h.a.cfg.Token != "" {
		t.Error("in-memory token should be cleared")
	}
}

func TestARBLogoutSaveFailure(t *testing.T) {
	arbBreakHome(t)
	h := arbNewHarness(t, nil, false)
	err := arbRun(t, logoutCmd(h.a))
	if err == nil || !strings.HasPrefix(err.Error(), "failed to update config file: ") {
		t.Fatalf("got %v", err)
	}
	arbAssertWrapped(t, err)
	if h.out.Len() != 0 {
		t.Errorf("got %q", h.out.String())
	}
}

func TestARBWhoamiNotAuthenticated(t *testing.T) {
	h := arbNewHarness(t, nil, false)
	h.a.cfg.Token = ""
	cmd := whoamiCmd(h.a)
	err := arbRun(t, cmd)
	if err == nil || err.Error() != "Not authenticated. Run 'bookbeam auth login' to authenticate." {
		t.Fatalf("got %v", err)
	}
	if !cmd.SilenceUsage {
		t.Error("usage should be silenced for the unauthenticated error")
	}
}

func TestARBWhoamiText(t *testing.T) {
	cases := []struct {
		name, body, team string
	}{
		{"named team", `{"name":"Ann","email":"ann@example.com","current_team":{"id":2,"name":"Acme"}}`, "Acme"},
		{"no team", `{"name":"Ann","email":"ann@example.com"}`, "Personal"},
		{"unnamed team", `{"name":"Ann","email":"ann@example.com","current_team":{"id":2,"name":""}}`, "Personal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var path, auth string
			h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
				path, auth = r.URL.Path, r.Header.Get("Authorization")
				arbWriteJSON(w, 200, tc.body)
			}, false)
			h.a.cfg.Host = "https://h.example.com"
			cmd := whoamiCmd(h.a)
			if err := arbRun(t, cmd); err != nil {
				t.Fatal(err)
			}
			if path != "/api/v1/user" || auth != "Bearer tok" {
				t.Errorf("request %s auth %q", path, auth)
			}
			want := "Property      Value\n" +
				"Name          Ann\n" +
				"Email         ann@example.com\n" +
				"Active Team   " + tc.team + "\n" +
				"API Host      https://h.example.com\n"
			if h.out.String() != want {
				t.Errorf("got\n%s\nwant\n%s", h.out.String(), want)
			}
			if cmd.SilenceUsage {
				t.Error("SilenceUsage should only be set when unauthenticated")
			}
		})
	}
}

func TestARBWhoamiJSONAndErrors(t *testing.T) {
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, `{"name":"Ann"}`)
	}, true)
	if err := arbRun(t, whoamiCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if h.out.String() != "{\n  \"name\": \"Ann\"\n}\n" {
		t.Errorf("got %q", h.out.String())
	}

	h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 401, `{"message":"Unauthenticated."}`)
	}, false)
	err := arbRun(t, whoamiCmd(h.a))
	if err == nil || err.Error() != "failed to get user details: API error (401): Unauthenticated." {
		t.Fatalf("got %v", err)
	}
	var api_err *client.APIError
	if !errors.As(err, &api_err) || api_err.StatusCode != 401 {
		t.Errorf("whoami should wrap the API error, got %v", err)
	}

	h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, `not json`)
	}, false)
	err = arbRun(t, whoamiCmd(h.a))
	if err == nil || !strings.HasPrefix(err.Error(), "failed to parse profile response: ") {
		t.Fatalf("got %v", err)
	}
	arbAssertWrapped(t, err)
	if h.out.Len() != 0 {
		t.Errorf("got %q", h.out.String())
	}
}

func TestARBBillingStatus(t *testing.T) {
	cases := []struct {
		name, body, access, plan string
	}{
		{"active", `{"has_access":true,"offer_type":"subscription","active_offer":"Pro","checkout_url":"https://pay.example.com"}`, "Active (Paid access granted)", "SUBSCRIPTION"},
		{"inactive", `{"has_access":false,"offer_type":"lifetime","active_offer":"Pro","checkout_url":"https://pay.example.com"}`, "Inactive (No access)", "LIFETIME"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				arbWriteJSON(w, 200, tc.body)
			}, false)
			if err := arbRun(t, billingStatusCmd(h.a)); err != nil {
				t.Fatal(err)
			}
			if path != "/api/v1/billing" {
				t.Errorf("path %s", path)
			}
			want := "Field           Value\n" +
				"Access Status   " + tc.access + "\n" +
				"Plan Type       " + tc.plan + "\n" +
				"Active Offer    Pro\n" +
				"Checkout Link   https://pay.example.com\n"
			if h.out.String() != want {
				t.Errorf("got\n%q\nwant\n%q", h.out.String(), want)
			}
		})
	}
}

func TestARBBillingStatusJSONAndErrors(t *testing.T) {
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, `{"has_access":true}`)
	}, true)
	if err := arbRun(t, billingStatusCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if h.out.String() != "{\n  \"has_access\": true\n}\n" {
		t.Errorf("got %q", h.out.String())
	}

	h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 403, `{"message":"Forbidden"}`)
	}, false)
	if err := arbRun(t, billingStatusCmd(h.a)); err == nil || err.Error() != "API error (403): Forbidden" {
		t.Errorf("got %v", err)
	}

	h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, `not json`)
	}, false)
	if err := arbRun(t, billingStatusCmd(h.a)); err == nil {
		t.Error("expected parse error")
	}
	if h.out.Len() != 0 {
		t.Errorf("got %q", h.out.String())
	}
}

func TestARBBillingCheckout(t *testing.T) {
	var path string
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		arbWriteJSON(w, 200, `{"checkout_url":"https://pay.example.com"}`)
	}, true)
	if err := arbRun(t, billingCheckoutCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/billing" {
		t.Errorf("path %s", path)
	}
	if h.out.String() != "{\n  \"checkout_url\": \"https://pay.example.com\"\n}\n" {
		t.Errorf("got %q", h.out.String())
	}

	for body, want := range map[string]string{
		`{"checkout_url":""}`: `checkout unavailable: {"checkout_url":""}`,
		`not json`:            "checkout unavailable: not json",
	} {
		h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
			arbWriteJSON(w, 200, body)
		}, true)
		if err := arbRun(t, billingCheckoutCmd(h.a)); err == nil || err.Error() != want {
			t.Errorf("body %q: got %v", body, err)
		}
		if h.out.Len() != 0 {
			t.Errorf("got %q", h.out.String())
		}
	}

	h = arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 500, `{"message":"down"}`)
	}, true)
	if err := arbRun(t, billingCheckoutCmd(h.a)); err == nil || err.Error() != "API error (500): down" {
		t.Errorf("got %v", err)
	}
}

func TestARBBillingCheckoutTextOpensBrowser(t *testing.T) {
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, `{"checkout_url":"https://pay.example.com"}`)
	}, false)
	if err := arbRun(t, billingCheckoutCmd(h.a)); err != nil {
		t.Fatal(err)
	}
	if h.out.String() != "Checkout URL: https://pay.example.com\n" {
		t.Errorf("got %q", h.out.String())
	}
	if len(h.opened) != 1 || h.opened[0] != "https://pay.example.com" {
		t.Errorf("opened %v", h.opened)
	}
}
