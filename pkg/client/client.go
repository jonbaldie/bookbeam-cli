package client

import (
	"bytes"
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

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
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
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Request(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	relPath := strings.TrimLeft(path, "/")
	fullURL := fmt.Sprintf("%s/%s", c.BaseURL, relPath)

	req, err := http.NewRequest(method, fullURL, body)
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

	resp, err := c.HTTPClient.Do(req)
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
	resp, err := c.Request(http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Post(path string, payload any) ([]byte, error) {
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

	resp, err := c.Request(http.MethodPost, path, bodyReader, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Put(path string, payload any) ([]byte, error) {
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

	resp, err := c.Request(http.MethodPut, path, bodyReader, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	resp, err := c.Request(http.MethodDelete, path, nil, "")
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
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fileFieldName, filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	for key, val := range fields {
		if err := writer.WriteField(key, val); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	resp, err := c.Request(http.MethodPost, path, body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
