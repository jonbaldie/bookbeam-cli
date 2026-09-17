package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClientNewTrimsTrailingSlash(t *testing.T) {
	c := New("https://api.bookbeam.app///", "token123")
	if c.BaseURL != "https://api.bookbeam.app" {
		t.Errorf("expected trimmed BaseURL, got %s", c.BaseURL)
	}
	if c.Token != "token123" {
		t.Errorf("expected token123, got %s", c.Token)
	}
	if c.Timeout != DefaultRequestTimeout {
		t.Errorf("expected DefaultRequestTimeout, got %v", c.Timeout)
	}
	if c.timeout() != DefaultRequestTimeout {
		t.Errorf("expected timeout() == DefaultRequestTimeout, got %v", c.timeout())
	}
	if c.httpClient() == nil {
		t.Errorf("expected non-nil httpClient()")
	}

	// Client with zero timeout and nil HTTPClient falls back to defaults
	emptyClient := &Client{}
	if emptyClient.timeout() != DefaultRequestTimeout {
		t.Errorf("expected fallback DefaultRequestTimeout, got %v", emptyClient.timeout())
	}
	if emptyClient.httpClient() != http.DefaultClient {
		t.Errorf("expected fallback http.DefaultClient, got %v", emptyClient.httpClient())
	}
}

func TestClientAuthenticationHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") != "Bearer test-secret-token" {
			t.Errorf("expected Bearer test-secret-token, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	c := New(ts.URL, "test-secret-token")
	data, err := c.Get("/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]string
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status ok, got %s", res["status"])
	}
}

func TestClientAPIErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "authorization_pending",
			"error_description": "The authorization request is still pending.",
		})
	}))
	defer ts.Close()

	c := New(ts.URL, "")
	_, err := c.Post("/oauth/device/token", map[string]string{"code": "123"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if apiErr.ErrorType != "authorization_pending" {
		t.Errorf("expected error authorization_pending, got %s", apiErr.ErrorType)
	}
}

func TestAPIErrorFormats(t *testing.T) {
	cases := []struct {
		err  APIError
		want string
	}{
		{
			err:  APIError{StatusCode: 400, ErrorType: "bad_request", ErrorDescription: "detail here"},
			want: "API error (400): bad_request - detail here",
		},
		{
			err:  APIError{StatusCode: 401, ErrorType: "unauthorized"},
			want: "API error (401): unauthorized",
		},
		{
			err:  APIError{StatusCode: 404, Message: "not found"},
			want: "API error (404): not found",
		},
		{
			err:  APIError{StatusCode: 500},
			want: "API error (500)",
		},
	}

	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestClientStatusCodeBoundary(t *testing.T) {
	// Status code 399 should NOT return APIError
	ts399 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(399)
		_, _ = w.Write([]byte(`{"code":399}`))
	}))
	defer ts399.Close()

	c := New(ts399.URL, "")
	data, err := c.Get("/test", nil)
	if err != nil {
		t.Fatalf("expected no error on 399, got: %v", err)
	}
	if string(data) != `{"code":399}` {
		t.Errorf("unexpected body: %s", string(data))
	}

	// Status code 400 SHOULD return APIError
	ts400 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer ts400.Close()

	c400 := New(ts400.URL, "")
	_, err = c400.Get("/test", nil)
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if _, ok := err.(*APIError); !ok {
		t.Errorf("expected *APIError, got %T", err)
	}
}

func TestClientGetQueryParameters(t *testing.T) {
	var capturedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.String()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := New(ts.URL, "")

	// Without query
	_, err := c.Get("/items", nil)
	if err != nil {
		t.Fatal(err)
	}
	if capturedPath != "/items" {
		t.Errorf("expected /items without query string, got: %s", capturedPath)
	}

	// With query
	query := url.Values{}
	query.Set("page", "2")
	query.Set("limit", "10")
	_, err = c.Get("/items", query)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(capturedPath, "/items?") || !strings.Contains(capturedPath, "page=2") {
		t.Errorf("expected query params in path, got: %s", capturedPath)
	}
}

func TestClientPostPutDelete(t *testing.T) {
	var capturedMethod, capturedContentType string
	var capturedBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedContentType = r.Header.Get("Content-Type")
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	c := New(ts.URL, "")

	// Post with payload
	_, err := c.Post("/create", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPost || capturedContentType != "application/json" {
		t.Errorf("unexpected method %s or content-type %s", capturedMethod, capturedContentType)
	}
	if !strings.Contains(string(capturedBody), `"name":"test"`) {
		t.Errorf("unexpected body: %s", string(capturedBody))
	}

	// Post without payload
	capturedContentType = ""
	capturedBody = nil
	_, err = c.Post("/empty-post", nil)
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPost || capturedContentType != "" {
		t.Errorf("unexpected content-type on nil payload: %s", capturedContentType)
	}
	if len(capturedBody) != 0 {
		t.Errorf("expected empty body, got: %s", string(capturedBody))
	}

	// Post with unmarshallable payload
	_, err = c.Post("/bad-payload", make(chan int))
	if err == nil {
		t.Fatal("expected error on unmarshallable payload, got nil")
	}

	// Put with payload
	_, err = c.Put("/update", map[string]string{"name": "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPut || capturedContentType != "application/json" {
		t.Errorf("unexpected method %s or content-type %s", capturedMethod, capturedContentType)
	}
	if !strings.Contains(string(capturedBody), `"name":"updated"`) {
		t.Errorf("unexpected body: %s", string(capturedBody))
	}

	// Put without payload
	capturedContentType = ""
	_, err = c.Put("/empty-put", nil)
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPut || capturedContentType != "" {
		t.Errorf("unexpected content-type on nil payload: %s", capturedContentType)
	}

	// Put with unmarshallable payload
	_, err = c.Put("/bad-payload", make(chan int))
	if err == nil {
		t.Fatal("expected error on unmarshallable payload, got nil")
	}

	// Delete
	_, err = c.Delete("/delete/1")
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got %s", capturedMethod)
	}
}

func TestShortAPICallsTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":"too_late"}`))
	}))
	defer ts.Close()

	c := New(ts.URL, "")
	c.Timeout = 25 * time.Millisecond

	// Test Get timeout
	_, err := c.Get("/slow-get", nil)
	if err == nil {
		t.Error("expected timeout error on Get, got nil")
	}

	// Test Post timeout
	_, err = c.Post("/slow-post", map[string]string{"k": "v"})
	if err == nil {
		t.Error("expected timeout error on Post, got nil")
	}

	// Test Put timeout
	_, err = c.Put("/slow-put", map[string]string{"k": "v"})
	if err == nil {
		t.Error("expected timeout error on Put, got nil")
	}

	// Test Delete timeout
	_, err = c.Delete("/slow-delete")
	if err == nil {
		t.Error("expected timeout error on Delete, got nil")
	}
}

func TestPostMultipartSuccess(t *testing.T) {
	var receivedFields = make(map[string]string)
	var receivedFileName string
	var receivedFileContent []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart content-type, got: %s", r.Header.Get("Content-Type"))
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}

		for k, v := range r.MultipartForm.Value {
			if len(v) > 0 {
				receivedFields[k] = v[0]
			}
		}

		files := r.MultipartForm.File["file"]
		if len(files) != 1 {
			t.Fatalf("expected 1 file part, got %d", len(files))
		}
		receivedFileName = files[0].Filename

		fileReader, err := files[0].Open()
		if err != nil {
			t.Fatalf("failed to open uploaded file part: %v", err)
		}
		defer fileReader.Close()
		receivedFileContent, _ = io.ReadAll(fileReader)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"filename":"test.pdf"}`))
	}))
	defer ts.Close()

	testFile := filepath.Join(t.TempDir(), "test.pdf")
	testData := []byte("%PDF-1.4\nTest file content")
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatal(err)
	}

	c := New(ts.URL, "auth-token")
	fields := map[string]string{
		"project_id": "101",
		"category":   "fiction",
	}

	resp, err := c.PostMultipart("/upload", fields, "file", testFile)
	if err != nil {
		t.Fatalf("PostMultipart failed: %v", err)
	}
	if !strings.Contains(string(resp), `"id":42`) {
		t.Errorf("unexpected response: %s", string(resp))
	}
	if receivedFileName != "test.pdf" {
		t.Errorf("expected filename test.pdf, got %s", receivedFileName)
	}
	if string(receivedFileContent) != string(testData) {
		t.Errorf("file content mismatch: got %q, want %q", receivedFileContent, testData)
	}
	if receivedFields["project_id"] != "101" || receivedFields["category"] != "fiction" {
		t.Errorf("fields mismatch: %+v", receivedFields)
	}
}

func TestPostMultipartFileNotFound(t *testing.T) {
	c := New("http://127.0.0.1:9999", "")
	_, err := c.PostMultipart("/upload", nil, "file", "/nonexistent/path/file.epub")
	if err == nil {
		t.Fatal("expected error on nonexistent file, got nil")
	}
}

func TestPostMultipartAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"Project not found"}`))
	}))
	defer ts.Close()

	testFile := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(testFile, []byte("epub content"), 0600); err != nil {
		t.Fatal(err)
	}

	c := New(ts.URL, "")
	_, err := c.PostMultipart("/projects/999/files", nil, "file", testFile)
	if err == nil {
		t.Fatal("expected APIError on 404, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected 404, got %d", apiErr.StatusCode)
	}
}

func TestPostMultipartStreamsWithoutLargeMemoryBuffering(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body in small chunks to simulate streaming receiver
		buf := make([]byte, 32*1024)
		for {
			_, err := r.Body.Read(buf)
			if err != nil {
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	// Create a 20MB temporary file
	testFile := filepath.Join(t.TempDir(), "large.pdf")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1024*1024)
	for i := 0; i < 20; i++ {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	c := New(ts.URL, "token")
	_, err = c.PostMultipart("/upload", nil, "file", testFile)
	if err != nil {
		t.Fatalf("PostMultipart failed: %v", err)
	}

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	totalAllocMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / (1024 * 1024)
	// A 20MB file buffered with bytes.Buffer would allocate >= 40 MB due to slice growth.
	// With streaming via io.Pipe, TotalAlloc during the upload is strictly bounded (< 5 MB).
	if totalAllocMB > 5.0 {
		t.Fatalf("PostMultipart allocated %.2f MB for a 20MB file; expected streamed upload with bounded memory (< 5 MB)", totalAllocMB)
	}
}
