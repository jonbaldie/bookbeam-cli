//go:build linux || darwin

package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
)

// fsizeHelperEnv names the config directory a re-executed test binary stores
// a token in under a tiny file-size limit; fsizeTokenEnv is that token.
const (
	fsizeHelperEnv = "BOOKBEAM_TEST_FSIZE_DIR"
	fsizeTokenEnv  = "BOOKBEAM_TEST_FSIZE_TOKEN"
)

// fsizeLimit is below the size of every config.json StoreToken writes here,
// so the write fails part-way through.
const fsizeLimit = 16

// fsizeResult is what the helper process reports about its StoreToken call.
type fsizeResult struct {
	Err   string
	Token string
}

// TestStoreTokenUnderFileSizeLimitHelper is not a test on its own: it runs
// StoreToken in a re-executed test binary whose file-size limit makes the
// write fail after any destination would have been opened.
func TestStoreTokenUnderFileSizeLimitHelper(t *testing.T) {
	dir := os.Getenv(fsizeHelperEnv)
	if dir == "" {
		t.Skip("helper process only")
	}
	s, err := Resolve(dir, envOf(nil), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGXFSZ)
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	lowered := limit
	lowered.Cur = fsizeLimit
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lowered); err != nil {
		t.Fatal(err)
	}
	storeErr := s.StoreToken(os.Getenv(fsizeTokenEnv))
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	result := fsizeResult{Token: s.Token()}
	if storeErr != nil {
		result.Err = storeErr.Error()
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		t.Fatal(err)
	}
}

// Regression test for #84: a write that fails after config.json would have
// been truncated leaves both config.json and the stored token unchanged.
func TestStoreTokenFailedWriteKeepsPreviousConfig(t *testing.T) {
	const previous = `{"host":"https://file.example","token":"old-token"}`
	for _, token := range []string{"new-token", ""} {
		t.Run("token="+token, func(t *testing.T) {
			dir := writeConfig(t, previous)
			cmd := exec.Command(os.Args[0], "-test.run=^TestStoreTokenUnderFileSizeLimitHelper$")
			cmd.Env = append(os.Environ(), fsizeHelperEnv+"="+dir, fsizeTokenEnv+"="+token)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("helper process: %v\n%s", err, out)
			}
			var result fsizeResult
			if err := json.Unmarshal(out[:jsonLineEnd(out)], &result); err != nil {
				t.Fatalf("helper output %q: %v", out, err)
			}
			if result.Err == "" {
				t.Fatal("StoreToken succeeded under the file-size limit")
			}
			if result.Token != "old-token" {
				t.Errorf("Token() after failed store = %q, want %q", result.Token, "old-token")
			}
			assertConfigUnchanged(t, dir, previous)
		})
	}
}

// jsonLineEnd is the length of the first line of out, which holds the
// helper's result ahead of the test framework's own output.
func jsonLineEnd(out []byte) int {
	for i, b := range out {
		if b == '\n' {
			return i
		}
	}
	return len(out)
}
