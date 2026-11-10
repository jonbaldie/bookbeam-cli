package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientDropsAuthorizationOnCrossOriginRedirect(t *testing.T) {
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "multiple authentication types")
			return
		}
		if r.URL.Query().Get("X-Amz-Signature") != "signed-object" {
			t.Error("redirect lost the storage signature")
		}
		_, _ = io.WriteString(w, "uploaded book bytes")
	}))
	defer storage.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer author-token" {
			t.Error("initial API request lost author authentication")
		}
		http.Redirect(w, r, storage.URL+"/book.epub?X-Amz-Signature=signed-object", http.StatusFound)
	}))
	defer api.Close()

	c := New(api.URL, "author-token")
	body, err := c.Get("/download", nil)
	if err != nil {
		t.Fatalf("signed storage download failed: %v", err)
	}
	if string(body) != "uploaded book bytes" {
		t.Fatalf("downloaded bytes = %q", body)
	}
}

func TestClientKeepsAuthorizationOnSameOriginRedirect(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer author-token" {
			t.Error("same-origin request lost author authentication")
		}
		if r.URL.Path == "/download" {
			http.Redirect(w, r, "/book.epub", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "uploaded book bytes")
	}))
	defer api.Close()

	c := New(api.URL, "author-token")
	body, err := c.Get("/download", nil)
	if err != nil || string(body) != "uploaded book bytes" {
		t.Fatalf("same-origin download = %q, %v", body, err)
	}
}

func TestClientRedirectCredentialsStayOnOrigin(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{"same origin", "https://example.com/book.epub", "Bearer author-token"},
		{"different port", "https://example.com:8443/book.epub", ""},
		{"different scheme", "http://example.com/book.epub", ""},
		{"subdomain", "https://storage.example.com/book.epub", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://example.com", "author-token")
			c.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/download" {
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{tc.target}},
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}
				if got := r.Header.Get("Authorization"); got != tc.want {
					t.Errorf("redirect Authorization = %q, want %q", got, tc.want)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("uploaded book bytes")),
				}, nil
			})
			body, err := c.Get("/download", nil)
			if err != nil || string(body) != "uploaded book bytes" {
				t.Fatalf("redirected download = %q, %v", body, err)
			}
		})
	}
}

func TestClientStopsRedirectLoops(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/download", http.StatusFound)
	}))
	defer api.Close()

	c := New(api.URL, "author-token")
	_, err := c.Get("/download", nil)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("redirect loop error = %v", err)
	}
}
