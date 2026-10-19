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

func TestDocumentAddsTopLevelFields(t *testing.T) {
	doc := responseDocument([]byte(`{"id":12345678901234567,"ratio":1.50,"extra":"kept"}`))
	doc.Set("api_host", "https://example.com")
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), `{"api_host":"https://example.com","extra":"kept","id":12345678901234567,"ratio":1.50}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestDocumentVisitsDataItems(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"array", `{"data":[{"slug":"one","extra":12345678901234567},{"slug":"two"}],"total":2}`, `{"data":[{"extra":12345678901234567,"index":0,"slug":"one"},{"index":1,"slug":"two"}],"total":2}`},
		{"object", `{"data":{"slug":"one"}}`, `{"data":{"index":0,"slug":"one"}}`},
		{"mixed array", `{"data":[null,3,{"slug":"three"}]}`, `{"data":[null,3,{"index":2,"slug":"three"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := responseDocument([]byte(tc.body))
			doc.EachItem(func(item map[string]any, i int) { item["index"] = i })
			out, err := json.Marshal(doc)
			if err != nil || string(out) != tc.want {
				t.Fatalf("got %s (%v), want %s", out, err, tc.want)
			}
		})
	}
}

func TestDocumentIgnoresUnsupportedResponses(t *testing.T) {
	for _, body := range []string{`plain ok`, ``, `[1,{"data":{}}]`, `null`, `42`, `"text"`} {
		doc := responseDocument([]byte(body))
		before, _ := json.Marshal(doc)
		doc.Set("added", true)
		doc.EachItem(func(map[string]any, int) { t.Fatal("visited non-object response") })
		after, err := json.Marshal(doc)
		if err != nil || string(after) != string(before) {
			t.Fatalf("%q changed: %s -> %s (%v)", body, before, after, err)
		}
	}
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":"text"}`, `{"data":[]}`} {
		doc := responseDocument([]byte(body))
		doc.EachItem(func(map[string]any, int) { t.Fatal("visited missing or non-object data") })
		out, err := json.Marshal(doc)
		if err != nil || string(out) != body {
			t.Fatalf("got %s (%v), want %s", out, err, body)
		}
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
	if got, _ := json.Marshal(responseDocument([]byte("plain ok"))); string(got) != `"plain ok"` {
		t.Fatalf("got %#v", got)
	}
	if got, _ := json.Marshal(responseDocument(nil)); string(got) != `""` {
		t.Fatalf("got %#v", got)
	}
}
