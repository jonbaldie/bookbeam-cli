package cmd

import (
	"bytes"
	"context"
	"errors"
	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type formalDownloadTransport func(*http.Request) (*http.Response, error)

func (f formalDownloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type formalFailRead struct{ sent bool }

func (r *formalFailRead) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("stream interrupted")
	}
	r.sent = true
	return copy(p, "partial"), nil
}
func TestFormalDownloadFailurePreservesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.epub")
	if err := os.WriteFile(path, []byte("valuable original contents"), 0600); err != nil {
		t.Fatal(err)
	}
	c := client.New("https://example.invalid", "")
	c.HTTPClient = &http.Client{Transport: formalDownloadTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(&formalFailRead{})}, nil
	})}
	var buf bytes.Buffer
	a := &app{apiCli: c, printer: &output.Printer{JSON: true, Out: &buf}}
	command := filesDownloadCmd(a)
	command.SetContext(context.Background())
	command.Flags().Set("output", path)
	err := command.RunE(command, []string{"1", "2"})
	if err == nil || !strings.Contains(err.Error(), "stream interrupted") {
		t.Fatalf("expected copy error, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "valuable original contents" {
		t.Fatalf("failed download changed destination, got %q", data)
	}
	if buf.Len() != 0 {
		t.Fatalf("failed download reported success %s", buf.String())
	}
}
func TestFormalExportFailurePreservesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.csv")
	os.WriteFile(path, []byte("keep"), 0600)
	c := client.New("https://example.invalid", "")
	c.HTTPClient = &http.Client{Transport: formalDownloadTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(&formalFailRead{})}, nil
	})}
	a := &app{apiCli: c, printer: &output.Printer{Out: io.Discard}}
	command := downloadersExportCmd(a)
	command.SetContext(context.Background())
	command.Flags().Set("output", path)
	if err := command.RunE(command, []string{"1"}); err == nil {
		t.Fatal("expected export read failure")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "keep" {
		t.Fatalf("export failure changed destination %q", data)
	}
}

type formalShortWriter struct{}

func (formalShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestFormalCSVStdoutShortWriteFails(t *testing.T) {
	c := client.New("https://example.invalid", "")
	c.HTTPClient = &http.Client{Transport: formalDownloadTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("csv payload"))}, nil
	})}
	a := &app{apiCli: c, printer: &output.Printer{Out: formalShortWriter{}}}
	command := downloadersExportCmd(a)
	command.SetContext(context.Background())
	command.Flags().Set("output", "-")
	var stderr bytes.Buffer
	command.SetErr(&stderr)
	if err := command.RunE(command, []string{"1"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected short write, got %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatal("failed stream printed confirmation")
	}
}
func TestFormalCSVSuccessfulPublicationAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		dir := t.TempDir()
		dest := filepath.Join(dir, "existing.csv")
		if err := os.WriteFile(dest, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		c := client.New("https://example.invalid", "")
		c.HTTPClient = &http.Client{Transport: formalDownloadTransport(func(r *http.Request) (*http.Response, error) {
			if cancelled {
				cancel()
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("complete"))}, nil
		})}
		_, err := saveDownloaderCSV(ctx, c, "/csv", dest)
		cancel()
		data, readErr := os.ReadFile(dest)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if cancelled {
			if !errors.Is(err, context.Canceled) || string(data) != "keep" {
				t.Fatalf("cancelled publication: %q %v", data, err)
			}
		} else {
			if err != nil || string(data) != "complete" {
				t.Fatalf("publication: %q %v", data, err)
			}
			info, _ := os.Stat(dest)
			if info.Mode().Perm() != 0600 {
				t.Fatalf("lost destination permissions: %v", info.Mode())
			}
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Fatalf("temporary file leaked: %v", entries)
		}
	}
}
