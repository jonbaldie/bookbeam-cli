//go:build unix

package client

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestFormalMultipartFailureEventuallyClosesFile(t *testing.T) {
	count := func() int {
		n := 0
		var s syscall.Stat_t
		for fd := 0; fd < 4096; fd++ {
			if syscall.Fstat(fd, &s) == nil {
				n++
			}
		}
		return n
	}
	p := t.TempDir() + "/book.epub"
	if err := os.WriteFile(p, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	c := New("http://[::1", "")
	before := count()
	if _, err := c.PostMultipart("/upload", nil, "file", p); err == nil {
		t.Fatal("expected malformed request")
	}
	end := time.Now().Add(2 * time.Second)
	for count() > before {
		if time.Now().After(end) {
			t.Fatal("producer file remains open after reader closes")
		}
		time.Sleep(time.Millisecond)
	}
}
