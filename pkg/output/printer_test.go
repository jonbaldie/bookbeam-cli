package output

import (
	"bytes"
	"errors"
	"os"
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

func TestPrintJSONIndentsWithTwoSpaces(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.PrintJSON(map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "{\n  \"a\": 1\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrintJSONReturnsEncodingError(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.PrintJSON(make(chan int)); err == nil {
		t.Fatal("expected error for unencodable value")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestPrintJSONReturnsWriteError(t *testing.T) {
	p := &Printer{Out: failingWriter{}}
	if err := p.PrintJSON(1); err == nil {
		t.Fatal("expected write error")
	}
}

func TestPrintRawJSONPrettyPrintsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.PrintRawJSON([]byte(`{"a":[1]}`)); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "{\n  \"a\": [\n    1\n  ]\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrintRawJSONPrintsInvalidJSONVerbatim(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	if err := p.PrintRawJSON([]byte("not json")); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "not json\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrintRawJSONReturnsWriteErrors(t *testing.T) {
	p := &Printer{Out: failingWriter{}}
	if err := p.PrintRawJSON([]byte(`{}`)); err == nil {
		t.Fatal("expected write error for valid JSON")
	}
	if err := p.PrintRawJSON([]byte("bad")); err == nil {
		t.Fatal("expected write error for invalid JSON")
	}
}

func TestPrintInfoOnlyInTextMode(t *testing.T) {
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
			p.PrintInfo("hello")
			if buf.String() != tc.want {
				t.Fatalf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestTableTextAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	p.Table([]string{"ID", "NAME"}, [][]string{{"1", "Alpha"}, {"22", "B"}})
	want := "ID   NAME\n1    Alpha\n22   B\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestTableJSONKeysCellsByHeaderAndDropsExtras(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	p.Table([]string{"ID", "NAME"}, [][]string{{"1", "Alpha", "extra"}})
	want := "[\n  {\n    \"ID\": \"1\",\n    \"NAME\": \"Alpha\"\n  }\n]\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestTableJSONWithNoRowsPrintsNull(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, JSON: true}
	p.Table([]string{"ID"}, nil)
	if buf.String() != "null\n" {
		t.Fatalf("got %q", buf.String())
	}
}
