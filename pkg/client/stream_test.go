package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type streamTransport func(*http.Request) (*http.Response, error)

func (f streamTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type failingSink struct{}

func (failingSink) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestStreamGet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		sink    io.Writer
		wantErr error
		wantN   int64
	}{
		{"success", 200, "first chunk;second chunk", &bytes.Buffer{}, nil, 24},
		{"sink failure", 200, "data", failingSink{}, io.ErrClosedPipe, 0},
		{"API failure", 422, `{"message":"invalid"}`, &bytes.Buffer{}, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			c := New("http://example.test", "token")
			c.HTTPClient.Transport = streamTransport(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Fatal("missing token")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"X-Test": {"value"}}, Body: body, Request: r}, nil
			})
			meta, n, err := StreamGet(c, context.Background(), "/csv", tc.sink)
			if !body.closed {
				t.Fatal("response body not closed")
			}
			if n != tc.wantN {
				t.Fatalf("bytes = %d, want %d", n, tc.wantN)
			}
			if tc.status == 422 {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != 422 || apiErr.Message != "invalid" {
					t.Fatalf("API error = %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if meta.StatusCode != 200 || meta.Header.Get("X-Test") != "value" {
				t.Fatalf("metadata = %+v", meta)
			}
			if b, ok := tc.sink.(*bytes.Buffer); ok && b.String() != tc.body {
				t.Fatalf("body = %q", b.String())
			}
		})
	}
}

func TestDefaultTransferTimeout(t *testing.T) {
	c := New("http://example.test", "")
	if DefaultTransferTimeout != 24*time.Hour || c.TransferTimeout != 24*time.Hour {
		t.Fatalf("transfer timeout = %v, default = %v", c.TransferTimeout, DefaultTransferTimeout)
	}
}

func TestStreamGetCancellationAndTransferTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
	}{
		{"caller cancellation", 0, true}, {"transfer timeout", 10 * time.Millisecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("http://example.test", "")
			c.Timeout = time.Nanosecond
			c.TransferTimeout = tc.timeout
			c.HTTPClient.Transport = streamTransport(func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				go func() { time.Sleep(10 * time.Millisecond); cancel() }()
			}
			_, _, err := StreamGet(c, ctx, "/csv", io.Discard)
			want := context.DeadlineExceeded
			if tc.cancel {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}
