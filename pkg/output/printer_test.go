package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNewWritesToStdoutWithGivenModes(t *testing.T) {
	p := New(true, true)
	if !p.JSON || !p.Quiet || p.Out != os.Stdout || p.In != os.Stdin {
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

func TestJSONOutputMasksNestedCredentials(t *testing.T) {
	for _, tc := range []struct {
		name  string
		print func(*Printer, any) error
	}{
		{"Display", func(p *Printer, data any) error { return p.Display(View{Data: data}) }},
		{"Success", func(p *Printer, data any) error { return p.Success(data, "") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{
				"nested": []any{map[string]any{
					"api_token":     "TEST_API_TOKEN_SENTINEL",
					"API_KEY":       "TEST_API_KEY_SENTINEL",
					"Token":         "TEST_TOKEN_SENTINEL",
					"sEcReT":        "TEST_SECRET_SENTINEL",
					"Password":      "TEST_PASSWORD_SENTINEL",
					"Authorization": "TEST_AUTHORIZATION_SENTINEL",
				}},
			}
			var buf bytes.Buffer
			p := &Printer{Out: &buf, JSON: true}
			if err := tc.print(p, data); err != nil {
				t.Fatal(err)
			}
			for _, sentinel := range []string{
				"TEST_API_TOKEN_SENTINEL", "TEST_API_KEY_SENTINEL", "TEST_TOKEN_SENTINEL",
				"TEST_SECRET_SENTINEL", "TEST_PASSWORD_SENTINEL", "TEST_AUTHORIZATION_SENTINEL",
			} {
				if bytes.Contains(buf.Bytes(), []byte(sentinel)) {
					t.Errorf("JSON output exposed credential %s: %s", sentinel, buf.String())
				}
			}
			if count := bytes.Count(buf.Bytes(), []byte(`"********"`)); count != 6 {
				t.Fatalf("JSON output contains %d masked values, want 6: %s", count, buf.String())
			}
			if got := data["nested"].([]any)[0].(map[string]any)["api_token"]; got != "TEST_API_TOKEN_SENTINEL" {
				t.Fatalf("printing mutated input credential: %#v", got)
			}
		})
	}
}

func TestJSONOutputMasksStructFieldsByJSONNameWithoutMutation(t *testing.T) {
	type nested struct {
		Secret string `json:"SeCrEt"`
	}
	type document struct {
		APIToken      string    `json:"Api_ToKeN"`
		APIKey        string    `json:"API_KEY"`
		Token         string    `json:"ToKeN"`
		Password      string    `json:"PASSWORD"`
		Authorization string    `json:"Authorization"`
		Nested        [1]nested `json:"nested"`
		Label         string    `json:"label"`
	}
	data := document{
		APIToken:      "STRUCT_API_TOKEN_SENTINEL",
		APIKey:        "STRUCT_API_KEY_SENTINEL",
		Token:         "STRUCT_TOKEN_SENTINEL",
		Password:      "STRUCT_PASSWORD_SENTINEL",
		Authorization: "STRUCT_AUTHORIZATION_SENTINEL",
		Nested:        [1]nested{{Secret: "STRUCT_SECRET_SENTINEL"}},
		Label:         "kept",
	}
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	if err := p.Success(data, ""); err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{
		"STRUCT_API_TOKEN_SENTINEL", "STRUCT_API_KEY_SENTINEL", "STRUCT_TOKEN_SENTINEL",
		"STRUCT_SECRET_SENTINEL", "STRUCT_PASSWORD_SENTINEL", "STRUCT_AUTHORIZATION_SENTINEL",
	} {
		if bytes.Contains(buf.Bytes(), []byte(sentinel)) {
			t.Errorf("JSON output exposed struct credential %s: %s", sentinel, buf.String())
		}
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	for _, key := range []string{"Api_ToKeN", "API_KEY", "ToKeN", "PASSWORD", "Authorization"} {
		if got[key] != "********" {
			t.Errorf("%s = %#v, want masked value", key, got[key])
		}
	}
	nestedGot, ok := got["nested"].([]any)
	if !ok || len(nestedGot) != 1 {
		t.Errorf("nested struct shape changed: %#v", got["nested"])
	} else if nestedObject, ok := nestedGot[0].(map[string]any); !ok || nestedObject["SeCrEt"] != "********" {
		t.Errorf("nested struct field was not masked: %#v", nestedGot[0])
	}
	if got["label"] != "kept" {
		t.Errorf("non-sensitive field changed: %#v", got["label"])
	}
	if data.APIToken != "STRUCT_API_TOKEN_SENTINEL" || data.Nested[0].Secret != "STRUCT_SECRET_SENTINEL" {
		t.Fatal("printing mutated the input struct")
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

func TestProceedAsksPeopleButTrustsForceAndScripts(t *testing.T) {
	const prompt = "Delete it? (y/N): "
	cases := []struct {
		name               string
		force, json, quiet bool
		in                 string
		want               string
		proceed            bool
	}{
		{"force", true, false, false, "n\n", "", true},
		{"json", false, true, false, "n\n", "", true},
		{"y", false, false, false, "y\n", prompt, true},
		{"padded YES", false, false, false, " YES \n", prompt, true},
		{"Y without newline", false, false, false, "Y", prompt, true},
		{"blank line", false, false, false, "\n", prompt + "Cancelled.\n", false},
		{"n", false, false, false, "n\n", prompt + "Cancelled.\n", false},
		{"empty", false, false, false, "", prompt + "Cancelled.\n", false},
		{"yep", false, false, false, "yep\n", prompt + "Cancelled.\n", false},
		{"quiet n", false, false, true, "n\n", prompt, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &Printer{Out: &buf, In: strings.NewReader(tc.in), JSON: tc.json, Quiet: tc.quiet}
			if got := p.Proceed(tc.force, "Delete it?"); got != tc.proceed || buf.String() != tc.want {
				t.Fatalf("proceed=%v got %q, want %v %q", got, buf.String(), tc.proceed, tc.want)
			}
		})
	}
}
