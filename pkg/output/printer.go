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
	Data        any    // encoded under --json, with credentials masked
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

// Launch reports like Success, then runs open only for a person: --json callers are scripts.
func (p *Printer) Launch(data any, msg string, open func()) error {
	if err := p.Success(data, msg); err != nil || p.JSON {
		return err
	}
	open()
	return nil
}

// Info prints msg for people; --quiet and --json suppress it.
func (p *Printer) Info(msg string) {
	if !p.Quiet && !p.JSON {
		fmt.Fprintln(p.Out, msg)
	}
}

// encode writes v as indented JSON with every credential masked, whatever command produced it.
func (p *Printer) encode(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if raw, err = maskCredentials(raw); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err = io.Copy(p.Out, &buf)
	return err
}

// credentialKeys are the object keys, matched case-insensitively, whose values never reach --json output.
var credentialKeys = []string{"api_token", "api_key", "token", "secret", "password", "authorization"}

const maskedCredential = `"********"`

// maskCredentials rewrites encoded JSON, replacing every non-empty value under a credential key
// at any depth. Working on the encoding covers maps, slices and structs alike, matches struct
// fields by their JSON names, and keeps key order, exact numbers and the caller's value intact.
func maskCredentials(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || (raw[0] != '{' && raw[0] != '[') {
		return raw, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	open, err := dec.Token()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteByte(raw[0])
	for i := 0; dec.More(); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		credential := false
		if open == json.Delim('{') {
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, _ := tok.(string)
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			out.Write(encodedKey)
			out.WriteByte(':')
			credential = isCredentialKey(key)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if credential && string(value) != "null" && string(value) != `""` {
			value = json.RawMessage(maskedCredential)
		} else if value, err = maskCredentials(value); err != nil {
			return nil, err
		}
		out.Write(value)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	out.WriteByte(raw[len(raw)-1])
	return out.Bytes(), nil
}

func isCredentialKey(key string) bool {
	for _, k := range credentialKeys {
		if strings.EqualFold(key, k) {
			return true
		}
	}
	return false
}

func writeRow(w io.Writer, cells []string) {
	fmt.Fprintln(w, strings.Join(cells, "\t"))
}
