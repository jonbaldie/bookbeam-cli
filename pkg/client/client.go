package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultRequestTimeout = 30 * time.Second

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type APIError struct {
	StatusCode       int    `json:"-"`
	Message          string `json:"message,omitempty"`
	ErrorType        string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("API error (%d): %s - %s", e.StatusCode, e.ErrorType, e.ErrorDescription)
	}
	if e.ErrorType != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.ErrorType)
	}
	if e.Message != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (%d)", e.StatusCode)
}

// New returns a client for baseURL, which must already be canonical: an http
// or https URL with no trailing slash, as config.Settings.Host returns.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{},
		Timeout:    DefaultRequestTimeout,
	}
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultRequestTimeout
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) Request(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	return c.RequestWithContext(context.Background(), method, path, body, contentType)
}

func (c *Client) RequestWithContext(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	relPath := strings.TrimLeft(path, "/")
	fullURL := fmt.Sprintf("%s/%s", c.BaseURL, relPath)

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		apiErr := &APIError{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(respBytes, apiErr)
		return nil, apiErr
	}

	return resp, nil
}

func (c *Client) Get(path string, query url.Values) ([]byte, error) {
	return c.GetWithContext(context.Background(), path, query)
}

// GetWithContext applies the short-request timeout within ctx's lifetime.
func (c *Client) GetWithContext(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if len(query) > 0 {
		path = fmt.Sprintf("%s?%s", path, query.Encode())
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Post(path string, payload any) ([]byte, error) {
	return c.PostWithContext(context.Background(), path, payload)
}

func (c *Client) Put(path string, payload any) ([]byte, error) {
	return c.PutWithContext(context.Background(), path, payload)
}

// PostWithContext sends JSON within ctx's lifetime and the short-request timeout.
func (c *Client) PostWithContext(ctx context.Context, path string, payload any) ([]byte, error) {
	return c.sendJSON(ctx, http.MethodPost, path, payload)
}

// PutWithContext sends JSON within ctx's lifetime and the short-request timeout.
func (c *Client) PutWithContext(ctx context.Context, path string, payload any) ([]byte, error) {
	return c.sendJSON(ctx, http.MethodPut, path, payload)
}

func (c *Client) sendJSON(ctx context.Context, method, path string, payload any) ([]byte, error) {
	var bodyReader io.Reader
	var contentType string

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
		contentType = "application/json"
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, method, path, bodyReader, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	return c.DeleteWithContext(context.Background(), path)
}

// DeleteWithContext applies the short-request timeout within ctx's lifetime.
func (c *Client) DeleteWithContext(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, http.MethodDelete, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) PostMultipart(path string, fields map[string]string, fileFieldName, filePath string) ([]byte, error) {
	return c.PostMultipartWithContext(context.Background(), path, fields, fileFieldName, filePath)
}

// PostMultipartWithContext streams a file with cancellation but no fixed transfer deadline.
func (c *Client) PostMultipartWithContext(ctx context.Context, path string, fields map[string]string, fileFieldName, filePath string) ([]byte, error) {
	return c.postMultipartWithProgress(ctx, path, fields, fileFieldName, filePath, nil)
}

// PostMultipartWithProgress reports cumulative file bytes handed to the transport.
func PostMultipartWithProgress(c *Client, ctx context.Context, path string, fields map[string]string, fileFieldName, filePath string, progress func(int64)) ([]byte, error) {
	return c.postMultipartWithProgress(ctx, path, fields, fileFieldName, filePath, progress)
}

func (c *Client) postMultipartWithProgress(ctx context.Context, path string, fields map[string]string, fileFieldName, filePath string, progress func(int64)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	defer pr.Close()
	writer := multipart.NewWriter(pw)
	updates := make(chan int64, 1)

	go func() {
		defer file.Close()
		pw.CloseWithError(writeMultipart(writer, file, fileFieldName, filePath, fields, progress != nil, updates))
	}()

	send := func() ([]byte, error) {
		resp, err := c.RequestWithContext(ctx, http.MethodPost, path, pr, writer.FormDataContentType())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	if progress == nil {
		return send()
	}
	return reportMultipartProgress(ctx, send, updates, progress)
}

func writeMultipart(writer *multipart.Writer, file *os.File, fieldName, filePath string, fields map[string]string, report bool, updates chan<- int64) error {
	part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return err
	}
	var dst io.Writer = part
	if report {
		dst = &progressWriter{writer: part, updates: updates}
	}
	if _, err = io.Copy(dst, file); err != nil {
		return err
	}
	for key, val := range fields {
		if err = writer.WriteField(key, val); err != nil {
			return err
		}
	}
	return writer.Close()
}

func reportMultipartProgress(ctx context.Context, send func() ([]byte, error), updates <-chan int64, progress func(int64)) ([]byte, error) {
	type result struct {
		body []byte
		err  error
	}
	finished := make(chan result, 1)
	go func() {
		body, err := send()
		finished <- result{body, err}
	}()
	for {
		select {
		case n := <-updates:
			progress(n)
		case r := <-finished:
			return r.body, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type progressWriter struct {
	writer  io.Writer
	updates chan<- int64
	bytes   int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.bytes += int64(n)
	if n > 0 {
		select {
		case w.updates <- w.bytes:
		default:
		}
	}
	return n, err
}
