package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewUsesThirtySecondTimeoutAndOwnHTTPClient(t *testing.T) {
	c := New("https://example.com/", "tok")
	if c.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.Timeout)
	}
	if DefaultRequestTimeout != 30*time.Second {
		t.Errorf("DefaultRequestTimeout = %v, want 30s", DefaultRequestTimeout)
	}
	if c.HTTPClient == nil || c.HTTPClient == http.DefaultClient {
		t.Errorf("expected a dedicated HTTP client, got %v", c.HTTPClient)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRequestsUseConfiguredHTTPClient(t *testing.T) {
	var gotURL string
	c := New("http://bookbeam.invalid", "")
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("custom")), Header: http.Header{}}, nil
	})}

	body, err := c.Get("/api/v1/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "custom" || gotURL != "http://bookbeam.invalid/api/v1/ping" {
		t.Fatalf("got body %q from %q", body, gotURL)
	}
}

func TestRequestsFallBackToDefaultHTTPClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("default"))
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}
	body, err := c.Get("/x", nil)
	if err != nil || string(body) != "default" {
		t.Fatalf("got %q, %v", body, err)
	}
}

func TestPositiveTimeoutBelowOneSecondIsHonoured(t *testing.T) {
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
	}))
	defer ts.Close()
	defer close(release)

	c := New(ts.URL, "")
	c.Timeout = time.Nanosecond
	start := time.Now()
	if _, err := c.Get("/slow", nil); err == nil {
		t.Fatal("expected a 1ns timeout to abort the request")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("request took %v, want the 1ns timeout honoured", elapsed)
	}
}

func TestRequestRejectsMalformedBaseURL(t *testing.T) {
	c := New("http://[::1", "")
	resp, err := c.Request(http.MethodGet, "/x", nil, "")
	if err == nil || resp != nil {
		t.Fatalf("expected URL error, got %v, %v", resp, err)
	}
}

func TestPostMultipartFailsWhenFileCannotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directories cannot be opened for reading on Windows")
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := New(ts.URL, "")
	if body, err := c.PostMultipart("/upload", map[string]string{"a": "b"}, "file", t.TempDir()); err == nil {
		t.Fatalf("expected read error to abort the upload, got body %q", body)
	}
}
