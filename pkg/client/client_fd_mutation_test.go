//go:build unix

package client

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func openFileDescriptors() int {
	count := 0
	var stat syscall.Stat_t
	for fd := 0; fd < 4096; fd++ {
		if syscall.Fstat(fd, &stat) == nil {
			count++
		}
	}
	return count
}

func TestPostMultipartClosesFileWhenRequestCannotStart(t *testing.T) {
	path := t.TempDir() + "/book.epub"
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	c := New("http://[::1", "")
	before := openFileDescriptors()

	if _, err := c.PostMultipart("/upload", map[string]string{"a": "b"}, "file", path); err == nil {
		t.Fatal("expected URL error")
	}

	deadline := time.Now().Add(2 * time.Second)
	for openFileDescriptors() > before {
		if time.Now().After(deadline) {
			t.Fatalf("file descriptor leaked: %d open, %d before", openFileDescriptors(), before)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Give the writer goroutine time to finish, so a crash after a failed part write surfaces here.
	time.Sleep(50 * time.Millisecond)
}
