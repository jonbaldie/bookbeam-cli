package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestProgressWriterReportsOnlyWrittenBytes(t *testing.T) {
	updates := make(chan int64, 2)
	w := &progressWriter{writer: io.Discard, updates: updates}
	if n, err := w.Write([]byte("book")); n != 4 || err != nil {
		t.Fatalf("write = %d, %v", n, err)
	}
	if n := <-updates; n != 4 {
		t.Fatalf("progress = %d, want 4", n)
	}
	w.writer = failedWriter{}
	if n, err := w.Write([]byte("file")); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed write = %d, %v", n, err)
	}
	select {
	case n := <-updates:
		t.Fatalf("reported %d bytes for a failed write", n)
	default:
	}
}

type pacedTransport struct{ final <-chan struct{} }

func (p pacedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	defer r.Body.Close()
	buf := make([]byte, 4096)
	for {
		_, err := r.Body.Read(buf)
		if err == io.EOF {
			select {
			case <-p.final:
			case <-time.After(time.Second):
			}
			break
		}
		if err != nil {
			return nil, err
		}
		time.Sleep(time.Millisecond)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(`{"id":56}`))}, nil
}

func TestMultipartProgressReportsExactFileBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.pdf")
	if err := os.WriteFile(path, make([]byte, 128*1024), 0600); err != nil {
		t.Fatal(err)
	}
	c := New("http://example.test", "")
	final := make(chan struct{})
	c.HTTPClient = &http.Client{Transport: pacedTransport{final: final}}
	var last int64
	_, err := PostMultipartWithProgress(c, context.Background(), "/upload", nil, "file", path, func(n int64) {
		last = n
		if n == 128*1024 {
			close(final)
		}
	})
	if err != nil || last != 128*1024 {
		t.Fatalf("last transferred count=%d err=%v, want 131072", last, err)
	}
}

func TestMultipartProgressReportsFileBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.pdf")
	if err := os.WriteFile(path, make([]byte, 128*1024), 0600); err != nil {
		t.Fatal(err)
	}
	progressSeen := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		part, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		file, err := part.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		n, err := io.Copy(io.Discard, file)
		if err != nil || n != 128*1024 {
			t.Errorf("received %d file bytes: %v", n, err)
		}
		select {
		case <-progressSeen:
		case <-time.After(3 * time.Second):
			t.Error("no progress before response")
		}
		fmt.Fprint(w, `{"id":56}`)
	}))
	defer server.Close()
	var updates []int64
	body, err := PostMultipartWithProgress(New(server.URL, ""), context.Background(), "/upload", nil, "file", path, func(n int64) {
		updates = append(updates, n)
		once.Do(func() { close(progressSeen) })
	})
	if err != nil || string(body) != `{"id":56}` {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if len(updates) == 0 {
		t.Fatal("no progress updates")
	}
	for i, n := range updates {
		if n <= 0 || n > 128*1024 || (i > 0 && n <= updates[i-1]) {
			t.Fatalf("invalid file byte updates: %v", updates)
		}
	}
}
