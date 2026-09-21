package output

import (
	"bytes"
	"encoding/json"
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
