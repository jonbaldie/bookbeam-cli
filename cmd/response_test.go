package cmd

import (
	"encoding/json"
	"testing"
)

func TestDecodeResponseFillsTypedAndKeepsFullDocument(t *testing.T) {
	var typed struct {
		ID int `json:"id"`
	}
	doc, err := decodeWithDocument([]byte(`{"id":12345678901234567,"ratio":1.50,"ok":true,"extra":"kept"}`), &typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.ID != 12345678901234567 {
		t.Fatalf("typed id %d", typed.ID)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), `{"extra":"kept","id":12345678901234567,"ok":true,"ratio":1.50}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestDecodeResponseRejectsBodiesTheTypeCannotHold(t *testing.T) {
	var typed struct {
		Data []int `json:"data"`
	}
	for _, body := range []string{`not json`, `{"data":"nope"}`} {
		if doc, err := decodeWithDocument([]byte(body), &typed); err == nil || doc != nil {
			t.Fatalf("%s: got %v %v", body, doc, err)
		}
	}
}

func TestResponseDocumentKeepsNonJSONBodiesAsText(t *testing.T) {
	if got := responseDocument([]byte("plain ok")); got != "plain ok" {
		t.Fatalf("got %#v", got)
	}
	if got := responseDocument(nil); got != "" {
		t.Fatalf("got %#v", got)
	}
}
