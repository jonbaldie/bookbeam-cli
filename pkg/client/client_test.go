package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
