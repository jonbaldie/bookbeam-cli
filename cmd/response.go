package cmd

import (
	"bytes"
	"encoding/json"
)

// document keeps the full API response while allowing commands to add derived fields.
type document struct {
	value any
}

// MarshalJSON serializes the underlying response, not the wrapper.
func (d *document) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.value)
}

// Set adds a top-level field to an object response; other responses are unchanged.
func (d *document) Set(key string, value any) {
	if object, ok := d.value.(map[string]any); ok {
		object[key] = value
	}
}

// EachItem visits object items in data, preserving their array indexes.
// A single data object is visited at index zero; other values are ignored.
func (d *document) EachItem(visit func(map[string]any, int)) {
	object, ok := d.value.(map[string]any)
	if !ok {
		return
	}
	switch data := object["data"].(type) {
	case map[string]any:
		visit(data, 0)
	case []any:
		for i, value := range data {
			if item, ok := value.(map[string]any); ok {
				visit(item, i)
			}
		}
	}
}

// decodeWithDocument fills typed from an API response and also returns the whole response as the --json document.
func decodeWithDocument(raw []byte, typed any) (*document, error) {
	if err := json.Unmarshal(raw, typed); err != nil {
		return nil, err
	}
	return responseDocument(raw), nil
}

// responseDocument decodes an API response without a schema, keeping every field and exact numbers; a non-JSON body stays text.
func responseDocument(raw []byte) *document {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return &document{value: string(raw)}
	}
	return &document{value: doc}
}
