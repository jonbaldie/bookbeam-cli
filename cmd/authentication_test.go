package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/spf13/cobra"
)

func TestNewCommandsRequireAuthenticationByDefault(t *testing.T) {
	for _, path := range [][]string{{"probe"}, {"group", "help"}, {"group", cobra.ShellCompRequestCmd}} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			root := NewRootCmd()
			ran := false
			leaf := &cobra.Command{Use: path[len(path)-1], RunE: func(*cobra.Command, []string) error {
				ran = true
				return nil
			}}
			if len(path) == 1 {
				root.AddCommand(leaf)
			} else {
				group := &cobra.Command{Use: path[0]}
				group.AddCommand(leaf)
				root.AddCommand(group)
			}
			arbIsolate(t)
			root.SetArgs(path)
			err := root.Execute()
			if err == nil || err.Error() != "Not authenticated. Run 'bookbeam auth login' to authenticate." || ran {
				t.Fatalf("new command bypassed authentication: ran=%v err=%v", ran, err)
			}
			if errors.Unwrap(err) != nil {
				t.Fatalf("missing-token error has an unexpected cause: %v", errors.Unwrap(err))
			}
		})
	}
}

func TestAuthenticationUsesEffectiveToken(t *testing.T) {
	for _, tc := range []struct {
		name, saved, env, flag, want string
	}{
		{"saved", "saved-token", "", "", "Bearer saved-token"},
		{"environment", "saved-token", "env-token", "", "Bearer env-token"},
		{"flag", "saved-token", "env-token", "flag-token", "Bearer flag-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var authorization string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization = r.Header.Get("Authorization")
				fmt.Fprint(w, `{"name":"Ann"}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			testSettingsIn(t, dir, server.URL, tc.saved)
			getenv := func(key string) string {
				return map[string]string{config.ConfigDirEnv: dir, "BOOKBEAM_TOKEN": tc.env}[key]
			}
			root := newRootCmd(getenv, func() (string, error) { return "", errors.New("home must not be used") })
			args := []string{"whoami"}
			if tc.flag != "" {
				args = append(args, "--token", tc.flag)
			}
			root.SetArgs(args)
			var err error
			out := arbCaptureStdout(t, func() { err = root.Execute() })
			if err != nil || authorization != tc.want || !strings.Contains(out, "Ann") {
				t.Fatalf("effective token not applied: auth=%q output=%q err=%v", authorization, out, err)
			}
		})
	}
}

func TestDeviceLogin401IsNotAnOAuthRetry(t *testing.T) {
	for _, errorType := range []string{"access_denied", "authorization_pending", "slow_down"} {
		t.Run(errorType, func(t *testing.T) {
			s := &arbDeviceServer{codeStatus: 200, codeBody: arbDeviceCode}
			s.reply(http.StatusUnauthorized, fmt.Sprintf(`{"error":%q}`, errorType))
			h := arbNewHarness(t, s.handler(t), false)
			command := loginCmd(h.a)
			wrapAuthenticationErrors(command)
			err := runCommand(command, nil)
			if err == nil || err.Error() != "Not authenticated. Run 'bookbeam auth login' to authenticate." || !command.SilenceUsage {
				t.Fatalf("expected a login hint instead of OAuth retries: %v", err)
			}
			var apiErr *client.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized || len(s.tokenPayloads) != 1 {
				t.Fatalf("lost the 401 or retried it: error=%v polls=%d", err, len(s.tokenPayloads))
			}
		})
	}
}

func TestAuthenticationErrorPreservesWrappedCause(t *testing.T) {
	cause := &client.APIError{StatusCode: http.StatusUnauthorized, Message: "revoked"}
	wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", cause))
	command := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return wrapped }}
	wrapAuthenticationErrors(command)
	command.SetArgs([]string{})
	err := command.Execute()
	var apiErr *client.APIError
	if err == nil || err.Error() != "Not authenticated. Run 'bookbeam auth login' to authenticate." || !errors.As(err, &apiErr) || apiErr != cause || errors.Unwrap(err) != wrapped {
		t.Fatalf("authentication presentation lost its cause: %v", err)
	}
}
