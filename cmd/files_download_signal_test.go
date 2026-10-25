package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
)

// Run the real download command in a child process, without catching signals.
func TestFilesDownloadSignalChild(t *testing.T) {
	host := os.Getenv("BOOKBEAM_TEST_DOWNLOAD_HOST")
	if host == "" {
		return
	}
	a := &app{apiCli: client.New(host, "test-token"), printer: &output.Printer{Out: io.Discard}}
	cmd := filesDownloadCmd(a)
	if err := cmd.Flags().Set("output", os.Getenv("BOOKBEAM_TEST_DOWNLOAD_OUTPUT")); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"7", "16"}); err != nil {
		t.Fatal(err)
	}
	t.Fatal("download unexpectedly completed")
}

func TestFilesDownloadSignalPreservesDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix interrupt and termination signals are not supported on Windows")
	}
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		for _, explicit := range []bool{false, true} {
			for _, existing := range []bool{false, true} {
				name := signal.String() + "/header/absent"
				if explicit {
					name = signal.String() + "/output/absent"
				}
				if existing {
					name += "/existing"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					dest := filepath.Join(dir, "keep.epub")
					const original = "complete original book"
					const partial = "partial incoming book"
					if existing {
						if err := os.WriteFile(dest, []byte(original), 0600); err != nil {
							t.Fatal(err)
						}
					}
					ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Disposition", `attachment; filename="keep.epub"`)
						w.Header().Set("Content-Length", "1000")
						_, _ = w.Write([]byte(partial))
						w.(http.Flusher).Flush()
						<-r.Context().Done()
					}))
					defer ts.Close()
					exe, err := os.Executable()
					if err != nil {
						t.Fatal(err)
					}
					child := exec.Command(exe, "-test.run=^TestFilesDownloadSignalChild$")
					child.Dir = dir
					outputPath := ""
					if explicit {
						outputPath = dest
					}
					child.Env = append(os.Environ(), "BOOKBEAM_TEST_DOWNLOAD_HOST="+ts.URL, "BOOKBEAM_TEST_DOWNLOAD_OUTPUT="+outputPath)
					var log bytes.Buffer
					child.Stdout, child.Stderr = &log, &log
					if err := child.Start(); err != nil {
						t.Fatal(err)
					}
					defer func() {
						_ = child.Process.Kill()
						_ = child.Wait()
					}()

					// Wait for actual bytes on disk, not a timing guess. This also
					// reaches the destructive write in the unfixed implementation.
					deadline := time.Now().Add(10 * time.Second)
					for {
						entries, err := os.ReadDir(dir)
						if err != nil {
							t.Fatal(err)
						}
						written := false
						for _, entry := range entries {
							data, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
							written = written || string(data) == partial
						}
						if written {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("download did not write partial bytes before deadline")
						}
						time.Sleep(5 * time.Millisecond)
					}
					if err := child.Process.Signal(signal); err != nil {
						t.Fatal(err)
					}
					if err := child.Wait(); err == nil {
						t.Fatalf("expected interrupted process, output=%s", log.String())
					}
					data, err := os.ReadFile(dest)
					if existing {
						if err != nil || string(data) != original {
							t.Fatalf("signal changed existing destination: data=%q err=%v", data, err)
						}
					} else if !os.IsNotExist(err) {
						t.Fatalf("signal left partial destination: data=%q err=%v", data, err)
					}
				})
			}
		}
	}
}
