package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// fieldEdit is one optional field: left alone, set to a value, or cleared.
type fieldEdit struct {
	value string
	clear bool
}

// editFlag reads a value flag and its paired clear flag, rejecting both together.
func editFlag(cmd *cobra.Command, name, clearName string) (fieldEdit, error) {
	value, _ := cmd.Flags().GetString(name)
	clear, _ := cmd.Flags().GetBool(clearName)
	if value != "" && clear {
		return fieldEdit{}, fmt.Errorf("cannot specify both --%s and --%s", name, clearName)
	}
	return fieldEdit{value: value, clear: clear}, nil
}

// putJSON writes the value, an explicit null for a clear, or nothing when unset.
func (e fieldEdit) putJSON(payload map[string]any, key string) {
	if e.clear {
		// The API keeps fields a PUT omits, so clearing needs an explicit null.
		payload[key] = nil
	} else if e.value != "" {
		payload[key] = e.value
	}
}

// putForm writes the value, an empty field for a clear, or nothing when unset.
func (e fieldEdit) putForm(fields map[string]string, key string) {
	if e.clear {
		// Multipart cannot carry a null; the API stores an empty field as null.
		fields[key] = ""
	} else if e.value != "" {
		fields[key] = e.value
	}
}

// unset reports whether the user neither set nor cleared the field.
func (e fieldEdit) unset() bool {
	return !e.clear && e.value == ""
}

// orExisting keeps the user's edit, or sets the field to its existing value when unset.
func (e fieldEdit) orExisting(existing string) fieldEdit {
	if e.unset() {
		e.value = existing
	}
	return e
}
