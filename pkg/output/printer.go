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

func (p *Printer) PrintJSON(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := fmt.Fprint(p.Out, buf.String())
	return err
}

func (p *Printer) PrintRawJSON(raw []byte) error {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		_, err = fmt.Fprintln(p.Out, string(raw))
		return err
	}
	_, err := fmt.Fprintln(p.Out, pretty.String())
	return err
}

func (p *Printer) PrintInfo(msg string) {
	if !p.Quiet && !p.JSON {
		fmt.Fprintln(p.Out, msg)
	}
}

func (p *Printer) Table(headers []string, rows [][]string) {
	if p.JSON {
		_ = p.PrintJSON(tableRecords(headers, rows))
		return
	}

	w := tabwriter.NewWriter(p.Out, 0, 0, 3, ' ', 0)
	writeRow(w, headers)
	for _, row := range rows {
		writeRow(w, row)
	}
	_ = w.Flush()
}

// tableRecords keys each row's cells by header; cells beyond the headers are dropped.
func tableRecords(headers []string, rows [][]string) []map[string]string {
	var records []map[string]string
	for _, row := range rows {
		record := make(map[string]string)
		for i, val := range row {
			if i < len(headers) {
				record[headers[i]] = val
			}
		}
		records = append(records, record)
	}
	return records
}

func writeRow(w io.Writer, cells []string) {
	fmt.Fprintln(w, strings.Join(cells, "\t"))
}
