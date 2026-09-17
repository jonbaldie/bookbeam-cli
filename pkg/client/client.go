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

func New(baseURL, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
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
		respBytes, _ := io.ReadAll(resp.Body)

		apiErr := &APIError{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(respBytes, apiErr)
		return nil, apiErr
	}

	return resp, nil
}

func (c *Client) Get(path string, query url.Values) ([]byte, error) {
	if len(query) > 0 {
		path = fmt.Sprintf("%s?%s", path, query.Encode())
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Post(path string, payload any) ([]byte, error) {
	return c.sendJSON(http.MethodPost, path, payload)
}

func (c *Client) Put(path string, payload any) ([]byte, error) {
	return c.sendJSON(http.MethodPut, path, payload)
}

func (c *Client) sendJSON(method, path string, payload any) ([]byte, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, method, path, bodyReader, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()

	resp, err := c.RequestWithContext(ctx, http.MethodDelete, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) PostMultipart(path string, fields map[string]string, fileFieldName, filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	defer pr.Close()
	writer := multipart.NewWriter(pw)

	go func() {
		var writeErr error
		defer func() {
			file.Close()
			if writeErr != nil {
				pw.CloseWithError(writeErr)
			} else {
				pw.Close()
			}
		}()

		part, err := writer.CreateFormFile(fileFieldName, filepath.Base(filePath))
		if err != nil {
			writeErr = err
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			writeErr = err
			return
		}

		for key, val := range fields {
			if err = writer.WriteField(key, val); err != nil {
				writeErr = err
				return
			}
		}

		writeErr = writer.Close()
	}()

	resp, err := c.Request(http.MethodPost, path, pr, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
