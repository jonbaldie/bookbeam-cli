package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestNewWritesToStdoutWithGivenModes(t *testing.T) {
	p := New(true, true)
	if !p.JSON || !p.Quiet || p.Out != os.Stdout {
		t.Fatalf("unexpected printer: %+v", p)
	}
	p = New(false, false)
	if p.JSON || p.Quiet {
		t.Fatalf("unexpected printer: %+v", p)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

var sampleView = View{
	Title:       "Things:",
	Headers:     []string{"ID", "NAME"},
	Rows:        [][]string{{"1", "Alpha"}, {"22", "B"}},
	Footer:      "Page 1 of 1",
	EmptyNotice: "Nothing here.",
	Data:        map[string]any{"id": 1, "ok": true, "name": "Alpha"},
}

func TestDisplayTextFramesAlignedTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.Display(sampleView); err != nil {
		t.Fatal(err)
	}
	want := "Things:\nID   NAME\n1    Alpha\n22   B\nPage 1 of 1\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestDisplayTextWithoutTitleOrFooterPrintsOnlyTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.Display(View{Headers: []string{"A"}, Rows: [][]string{{"x"}}}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "A\nx\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDisplayTextWithoutRowsPrintsEmptyNoticeInsteadOfTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	v := sampleView
	v.Rows = nil
	if err := p.Display(v); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "Nothing here.\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDisplayJSONEncodesTypedDataWithTwoSpaceIndent(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	if err := p.Display(sampleView); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"id\": 1,\n  \"name\": \"Alpha\",\n  \"ok\": true\n}\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestDisplayJSONKeepsDecodedNumbersExact(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	data := map[string]any{"big": json.Number("12345678901234567"), "price": json.Number("1.50")}
	if err := p.Display(View{Data: data}); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"big\": 12345678901234567,\n  \"price\": 1.50\n}\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestDisplayJSONWithEmptySliceEmitsEmptyArrayNotNotice(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true, Quiet: true}
	if err := p.Display(View{EmptyNotice: "Nothing here.", Data: []int{}}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "[]\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDisplayQuietKeepsTableButDropsInfoLines(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Quiet: true}
	if err := p.Display(sampleView); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "ID   NAME\n1    Alpha\n22   B\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	buf.Reset()
	v := sampleView
	v.Rows = nil
	if err := p.Display(v); err != nil || buf.Len() != 0 {
		t.Fatalf("quiet empty notice: %v %q", err, buf.String())
	}
}

func TestDisplayReturnsEncodingAndWriteErrors(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	if err := p.Display(View{Data: make(chan int)}); err == nil || buf.Len() != 0 {
		t.Fatalf("unencodable data: %v %q", err, buf.String())
	}
	if err := (&Printer{Out: failingWriter{}, JSON: true}).Display(sampleView); err == nil {
		t.Fatal("expected JSON write error")
	}
	if err := (&Printer{Out: failingWriter{}, Quiet: true}).Display(sampleView); err == nil {
		t.Fatal("expected table write error")
	}
}

func TestSuccessPrintsMessageOrData(t *testing.T) {
	cases := []struct {
		name        string
		json, quiet bool
		want        string
	}{
		{"text", false, false, "✓ Done\n"},
		{"quiet", false, true, ""},
		{"json", true, false, "{\n  \"id\": 7\n}\n"},
		{"json quiet", true, true, "{\n  \"id\": 7\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &Printer{Out: &buf, JSON: tc.json, Quiet: tc.quiet}
			if err := p.Success(map[string]int{"id": 7}, "✓ Done"); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.want {
				t.Fatalf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestInfoOnlyInTextMode(t *testing.T) {
	cases := []struct {
		name        string
		json, quiet bool
		want        string
	}{
		{"text", false, false, "hello\n"},
		{"json", true, false, ""},
		{"quiet", false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &Printer{Out: &buf, JSON: tc.json, Quiet: tc.quiet}
			p.Info("hello")
			if buf.String() != tc.want {
				t.Fatalf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestLaunchOpensOnlyForPeopleAfterReporting(t *testing.T) {
	cases := []struct {
		name        string
		json, quiet bool
		want        string
		opened      bool
	}{
		{"text", false, false, "Go\nopened\n", true},
		{"quiet", false, true, "opened\n", true},
		{"json", true, false, "{\n  \"id\": 7\n}\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &Printer{Out: &buf, JSON: tc.json, Quiet: tc.quiet}
			opened := false
			err := p.Launch(map[string]int{"id": 7}, "Go", func() { opened = true; buf.WriteString("opened\n") })
			if err != nil || opened != tc.opened || buf.String() != tc.want {
				t.Fatalf("err=%v opened=%v got %q, want %q", err, opened, buf.String(), tc.want)
			}
		})
	}
}

func TestLaunchSkipsOpenWhenReportFails(t *testing.T) {
	p := &Printer{Out: failingWriter{}, JSON: true}
	opened := false
	if err := p.Launch(map[string]int{"id": 7}, "Go", func() { opened = true }); err == nil || opened {
		t.Fatalf("err=%v opened=%v", err, opened)
	}
}

type providerConfig struct {
	URL   string `json:"api_url"`
	Token string `json:"API_Token"`
}

type sensitiveDoc struct {
	Name     string            `json:"name"`
	Password string            `json:"password,omitempty"`
	Teams    []map[string]any  `json:"teams"`
	Config   *providerConfig   `json:"config"`
	Headers  map[string]string `json:"headers"`
}

func sensitiveFixture() sensitiveDoc {
	return sensitiveDoc{
		Name:     "Author <a&b>",
		Password: "sentinel-password",
		Teams: []map[string]any{
			{"id": json.Number("12345678901234567"), "settings": map[string]any{"api_key": "sentinel-key", "Secret": []any{"sentinel-secret"}, "token": "", "api_token": nil}},
		},
		Config:  &providerConfig{URL: "https://provider.example/api", Token: "sentinel-token"},
		Headers: map[string]string{"Authorization": "Bearer sentinel-bearer"},
	}
}

const sensitiveWant = `{
  "name": "Author \u003ca\u0026b\u003e",
  "password": "********",
  "teams": [
    {
      "id": 12345678901234567,
      "settings": {
        "Secret": "********",
        "api_key": "********",
        "api_token": null,
        "token": ""
      }
    }
  ],
  "config": {
    "api_url": "https://provider.example/api",
    "API_Token": "********"
  },
  "headers": {
    "Authorization": "********"
  }
}
`

func TestJSONMasksSensitiveKeysThroughDisplayAndSuccess(t *testing.T) {
	for _, tc := range []struct {
		name  string
		print func(*Printer, any) error
	}{
		{"display", func(p *Printer, data any) error { return p.Display(View{Rows: [][]string{{"x"}}, Data: data}) }},
		{"success", func(p *Printer, data any) error { return p.Success(data, "✓ Done") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			data := sensitiveFixture()
			if err := tc.print(&Printer{Out: &buf, JSON: true}, data); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != sensitiveWant {
				t.Fatalf("got %s, want %s", got, sensitiveWant)
			}
			if !reflect.DeepEqual(data, sensitiveFixture()) {
				t.Fatalf("printing mutated its input: %#v", data)
			}
		})
	}
}

func TestJSONMasksSensitiveKeysInDecodedDocumentsWithoutMutatingThem(t *testing.T) {
	doc := map[string]any{"user": map[string]any{"name": "Author", "token": "sentinel-token"}}
	var buf bytes.Buffer
	if err := (&Printer{Out: &buf, JSON: true}).Success(doc, ""); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"user\": {\n    \"name\": \"Author\",\n    \"token\": \"********\"\n  }\n}\n"; buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
	if doc["user"].(map[string]any)["token"] != "sentinel-token" {
		t.Fatalf("printing mutated its input: %#v", doc)
	}
}

func TestJSONLeavesScalarsAndKeysThatOnlyResembleSecretsAlone(t *testing.T) {
	for _, tc := range []struct {
		data any
		want string
	}{
		{"token", "\"token\"\n"},
		{nil, "null\n"},
		{[]any{"secret", 1.5}, "[\n  \"secret\",\n  1.5\n]\n"},
		{map[string]any{"token_type": "Bearer", "tokens": 3, "passwordless": true}, "{\n  \"passwordless\": true,\n  \"token_type\": \"Bearer\",\n  \"tokens\": 3\n}\n"},
	} {
		var buf bytes.Buffer
		if err := (&Printer{Out: &buf, JSON: true}).Success(tc.data, ""); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tc.want {
			t.Errorf("got %q, want %q", buf.String(), tc.want)
		}
	}
}

func TestTextOutputIsNotMasked(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.Display(View{Headers: []string{"token"}, Rows: [][]string{{"visible"}}, Data: map[string]any{"token": "x"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "token\nvisible\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
