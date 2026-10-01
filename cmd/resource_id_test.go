package cmd

import (
	"strings"
	"testing"
)

func TestResourceIDParsesOnlyPositiveIntegers(t *testing.T) {
	cases := []struct {
		arg  string
		want int
		ok   bool
	}{
		{"7", 7, true},
		{"16", 16, true},
		{"0", 0, false},
		{"-1", 0, false},
		{"+7", 0, false},
		{"07", 0, false},
		{"7/files/3", 0, false},
		{"3/../5", 0, false},
		{"7?x=1", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{" 5", 0, false},
		{"5 ", 0, false},
		{"99999999999999999999", 0, false},
	}
	for _, tc := range cases {
		got, err := resourceID(tc.arg, "link")
		if tc.ok {
			if err != nil || got != tc.want {
				t.Errorf("resourceID(%q) = %d, %v; want %d, nil", tc.arg, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("resourceID(%q) = %d, nil; want error", tc.arg, got)
			continue
		}
		if want := `invalid link id "` + tc.arg + `"`; err.Error() != want {
			t.Errorf("resourceID(%q) error = %q; want %q", tc.arg, err.Error(), want)
		}
	}
}

// Every command that takes an ID must reject a malformed one before any request is sent.
func TestMalformedResourceIDsSendNoRequests(t *testing.T) {
	upload := flpWriteFile(t, "book.epub", "epub")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"projects", "get", "7?x=1"}, `invalid project id "7?x=1"`},
		{[]string{"projects", "update", "7/files/3", "--title", "T"}, `invalid project id "7/files/3"`},
		{[]string{"projects", "delete", "--force", "7/files/3"}, `invalid project id "7/files/3"`},
		{[]string{"projects", "newsletter", "abc", "--list-id", "L"}, `invalid project id "abc"`},
		{[]string{"links", "list", "7/.."}, `invalid project id "7/.."`},
		{[]string{"links", "create", "0"}, `invalid project id "0"`},
		{[]string{"links", "update", "7", " 5", "--title", "New"}, `invalid link id " 5"`},
		{[]string{"links", "update", "x", "5", "--title", "New"}, `invalid project id "x"`},
		{[]string{"links", "delete", "--force", "7", "5/../6"}, `invalid link id "5/../6"`},
		{[]string{"links", "delete", "--force", "--", "-7", "5"}, `invalid project id "-7"`},
		{[]string{"files", "list", "7?x"}, `invalid project id "7?x"`},
		{[]string{"files", "upload", "7/links", upload}, `invalid project id "7/links"`},
		{[]string{"files", "download", "7", "3/../../../links/5"}, `invalid file id "3/../../../links/5"`},
		{[]string{"files", "download", "", "3"}, `invalid project id ""`},
		{[]string{"files", "delete", "--force", "7", "3/../../../links/5"}, `invalid file id "3/../../../links/5"`},
		{[]string{"files", "delete", "--force", "7#", "3"}, `invalid project id "7#"`},
		{[]string{"downloaders", "list", "7/links"}, `invalid project id "7/links"`},
		{[]string{"downloaders", "export", "../7"}, `invalid project id "../7"`},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			ts, seen := ntdServer(t, 200, `{"data":{}}`)
			flpChdirTemp(t)
			_, err := ntdRoot(t, append([]string{"--host", ts.URL, "--token", "tok"}, tc.args...)...)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("want error %q, got %v", tc.want, err)
			}
			if len(*seen) != 0 {
				t.Fatalf("want zero requests, got %+v", *seen)
			}
		})
	}
}
