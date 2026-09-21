package cmd

import (
	"bytes"
	"encoding/json"
)

// decodeResponse fills typed from an API response and returns the whole response as the --json document.
func decodeResponse(raw []byte, typed any) (any, error) {
	if err := json.Unmarshal(raw, typed); err != nil {
		return nil, err
	}
	return responseDocument(raw), nil
}

// responseDocument decodes an API response without a schema, keeping every field and exact numbers; a non-JSON body stays text.
func responseDocument(raw []byte) any {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return string(raw)
	}
	return doc
}
