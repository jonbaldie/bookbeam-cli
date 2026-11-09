package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type formalTransport func(*http.Request) (*http.Response, error)

func (f formalTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type formalBody struct {
	io.Reader
	closed bool
}

func (b *formalBody) Close() error { b.closed = true; return nil }

func TestFormalHTTPDeadlineAndOwnership(t *testing.T) {
	for _, status := range []int{200, 399, 400, 500} {
		for _, short := range []bool{false, true} {
			b := &formalBody{Reader: bytes.NewBufferString(`{}`)}
			c := New("https://example.invalid", "")
			c.HTTPClient = &http.Client{Transport: formalTransport(func(r *http.Request) (*http.Response, error) {
				_, deadline := r.Context().Deadline()
				if deadline != short {
					t.Errorf("status %d short %v deadline %v", status, short, deadline)
				}
				return &http.Response{StatusCode: status, Body: b, Header: make(http.Header)}, nil
			})}
			if short {
				_, _ = c.Get("/x", nil)
				if !b.closed {
					t.Error("Get left response open")
				}
			} else {
				resp, err := c.Request("GET", "/x", nil, "")
				if status >= 400 {
					if err == nil || !b.closed {
						t.Error("HTTP error response must be closed by client")
					}
				} else {
					if err != nil || b.closed {
						t.Error("successful raw response belongs to caller")
					}
					if resp != nil {
						resp.Body.Close()
					}
				}
			}
		}
	}
}

func TestFormalMultipartBoundaryAndFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(file, []byte("book payload"), 0600); err != nil {
		t.Fatal(err)
	}
	c := New("https://example.invalid", "")
	c.HTTPClient = &http.Client{Transport: formalTransport(func(r *http.Request) (*http.Response, error) {
		if _, has := r.Context().Deadline(); has {
			t.Error("multipart unexpectedly has short deadline")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		got, _, err := r.FormFile("book")
		if err != nil {
			t.Fatal(err)
		}
		defer got.Close()
		data, _ := io.ReadAll(got)
		if string(data) != "book payload" || r.FormValue("title") != "Test" {
			t.Error("multipart lost file or field")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(`{}`))}, nil
	})}
	if _, err := c.PostMultipart("/upload", map[string]string{"title": "Test"}, "book", file); err != nil {
		t.Fatal(err)
	}
}

func TestFormalTransferDeadlineAndCancelledRequest(t *testing.T) {
	c := New("https://example.invalid", "")
	if c.TransferTimeout != 24*time.Hour {
		t.Fatalf("transfer default %v", c.TransferTimeout)
	}
	calls := 0
	c.HTTPClient = &http.Client{Transport: formalTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		deadline, ok := r.Context().Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining < 23*time.Hour || remaining > 24*time.Hour {
			t.Fatalf("transfer deadline %v %v", deadline, ok)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("payload"))}, nil
	})}
	var dst bytes.Buffer
	if _, n, err := StreamGet(c, context.Background(), "/csv", &dst); err != nil || n != 7 {
		t.Fatalf("stream %d %v", n, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.RequestWithContext(ctx, "GET", "/raw", nil, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request %v", err)
	}
	if calls != 1 {
		t.Fatalf("cancelled request reached transport %d", calls)
	}
}
func TestFormalMultipartProgressIsBoundedAndActual(t *testing.T) {
	updates := make(chan int64, 1)
	w := &progressWriter{writer: io.Discard, updates: updates}
	if n, err := w.Write([]byte("one")); n != 3 || err != nil {
		t.Fatal(n, err)
	}
	done := make(chan struct{})
	go func() { w.Write([]byte("two")); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("full progress channel blocked producer")
	}
	if n := <-updates; n != 3 {
		t.Fatalf("progress %d", n)
	}
	if w.bytes != 6 {
		t.Fatalf("transport byte count %d", w.bytes)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release := make(chan struct{})
	finished := make(chan struct{})
	defer close(release)
	_, err := reportMultipartProgress(ctx, func() ([]byte, error) { defer close(finished); <-release; return nil, nil }, updates, func(int64) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled progress %v", err)
	}
}
