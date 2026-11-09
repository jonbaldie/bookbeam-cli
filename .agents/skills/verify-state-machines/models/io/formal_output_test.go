package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type formalWriter struct {
	bytes.Buffer
	fail bool
}

func (w *formalWriter) Write(p []byte) (int, error) {
	if w.fail {
		return 0, errors.New("output unavailable")
	}
	return w.Buffer.Write(p)
}
func TestFormalOutputPreparationAndLaunch(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		for _, quiet := range []bool{false, true} {
			var buf bytes.Buffer
			p := &Printer{JSON: jsonMode, Quiet: quiet, Out: &buf}
			opened := false
			if err := p.Launch(map[string]string{"ok": "yes"}, "ready", func() { opened = true }); err != nil {
				t.Fatal(err)
			}
			if opened == jsonMode {
				t.Fatalf("JSON %v quiet %v opened %v", jsonMode, quiet, opened)
			}
		}
	}
	var buf bytes.Buffer
	p := &Printer{JSON: true, Out: &buf}
	if err := p.Success(make(chan int), ""); err == nil || buf.Len() != 0 {
		t.Fatal("preparation failure published bytes or returned success")
	}
	opened := false
	p.Out = &formalWriter{fail: true}
	if err := p.Launch(map[string]int{"ok": 1}, "ready", func() { opened = true }); err == nil || opened {
		t.Fatal("failed output launched browser")
	}
}
func TestFormalOutputFlushFailureStopsFooter(t *testing.T) {
	w := &formalWriter{fail: true}
	p := &Printer{Out: w}
	if err := p.Display(View{Headers: []string{"id"}, Rows: [][]string{{"1"}}, Footer: "must not print"}); err == nil {
		t.Fatal("flush failure lost")
	}
	if strings.Contains(w.String(), "must not print") {
		t.Fatal("footer after failure")
	}
}

func TestFormalProceedJSONAuthorizesWithoutInput(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{JSON: true, Out: &out, In: strings.NewReader("")}
	if !p.Proceed(false, "Delete?") {
		t.Fatal("JSON script unexpectedly refused")
	}
	if out.Len() != 0 {
		t.Fatal("JSON script was prompted")
	}
}
