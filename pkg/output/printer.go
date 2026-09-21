package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

type Printer struct {
	JSON  bool
	Quiet bool
	Out   io.Writer
}

func New(jsonOutput, quiet bool) *Printer {
	return &Printer{
		JSON:  jsonOutput,
		Quiet: quiet,
		Out:   os.Stdout,
	}
}

// View is one command result: a table for people and a JSON document for scripts.
type View struct {
	Title       string // info line above a non-empty table
	Headers     []string
	Rows        [][]string
	Footer      string // info line below a non-empty table, e.g. pagination
	EmptyNotice string // info line shown instead of a table without rows
	Data        any    // encoded as-is under --json
}

// Display prints v as JSON under --json, otherwise as a table framed by its info lines.
func (p *Printer) Display(v View) error {
	if p.JSON {
		return p.encode(v.Data)
	}
	if len(v.Rows) == 0 {
		p.Info(v.EmptyNotice)
		return nil
	}

	if v.Title != "" {
		p.Info(v.Title)
	}
	w := tabwriter.NewWriter(p.Out, 0, 0, 3, ' ', 0)
	writeRow(w, v.Headers)
	for _, row := range v.Rows {
		writeRow(w, row)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if v.Footer != "" {
		p.Info(v.Footer)
	}
	return nil
}

// Success reports a completed action: data under --json, otherwise msg on the info channel.
func (p *Printer) Success(data any, msg string) error {
	if p.JSON {
		return p.encode(data)
	}
	p.Info(msg)
	return nil
}

// Info prints msg for people; --quiet and --json suppress it.
func (p *Printer) Info(msg string) {
	if !p.Quiet && !p.JSON {
		fmt.Fprintln(p.Out, msg)
	}
}

func (p *Printer) encode(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := io.Copy(p.Out, &buf)
	return err
}

func writeRow(w io.Writer, cells []string) {
	fmt.Fprintln(w, strings.Join(cells, "\t"))
}
