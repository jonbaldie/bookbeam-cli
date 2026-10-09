package cmd

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func fieldEditCmd(t *testing.T, flags map[string]string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{}
	c.Flags().String("tags", "", "")
	c.Flags().Bool("clear-tags", false, "")
	for k, v := range flags {
		if err := c.Flags().Set(k, v); err != nil {
			t.Fatalf("set flag %s: %v", k, err)
		}
	}
	return c
}

func TestFieldEditEncodesUnsetSetAndCleared(t *testing.T) {
	cases := []struct {
		name     string
		flags    map[string]string
		wantJSON map[string]any
		wantForm map[string]string
	}{
		{"unset", nil, map[string]any{}, map[string]string{}},
		{"set", map[string]string{"tags": "a,b"}, map[string]any{"k": "a,b"}, map[string]string{"k": "a,b"}},
		{"cleared", map[string]string{"clear-tags": "true"}, map[string]any{"k": nil}, map[string]string{"k": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := editFlag(fieldEditCmd(t, tc.flags), "tags", "clear-tags")
			if err != nil {
				t.Fatal(err)
			}
			payload := map[string]any{}
			e.putJSON(payload, "k")
			if !reflect.DeepEqual(payload, tc.wantJSON) {
				t.Errorf("putJSON = %#v, want %#v", payload, tc.wantJSON)
			}
			fields := map[string]string{}
			e.putForm(fields, "k")
			if !reflect.DeepEqual(fields, tc.wantForm) {
				t.Errorf("putForm = %#v, want %#v", fields, tc.wantForm)
			}
		})
	}
}

func TestEditFlagRejectsValueWithClear(t *testing.T) {
	_, err := editFlag(fieldEditCmd(t, map[string]string{"tags": "a", "clear-tags": "true"}), "tags", "clear-tags")
	if err == nil || err.Error() != "cannot specify both --tags and --clear-tags" {
		t.Fatalf("err = %v", err)
	}
}
