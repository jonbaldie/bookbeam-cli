package output

import (
	"bytes"
	"encoding/json"
	"strings"
)

const maskedCredential = "********"

var sensitiveKeys = [...]string{
	"api_token",
	"api_key",
	"token",
	"secret",
	"password",
	"authorization",
}

// sanitize returns a copy of data with values under sensitive JSON keys masked.
// The JSON round trip applies struct tags and custom marshalers without mutating data.
func sanitize(data any) (any, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	return sanitizeJSONValue(document), nil
}

func sanitizeJSONValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, field := range value {
			if isSensitiveKey(key) {
				value[key] = maskedCredential
			} else {
				value[key] = sanitizeJSONValue(field)
			}
		}
	case []any:
		for i, field := range value {
			value[i] = sanitizeJSONValue(field)
		}
	}
	return value
}

func isSensitiveKey(key string) bool {
	for _, sensitiveKey := range sensitiveKeys {
		if strings.EqualFold(key, sensitiveKey) {
			return true
		}
	}
	return false
}
