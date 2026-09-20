package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type flpRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
	Fields map[string]string
	File   string
	Data   string
}

// flpServe starts a server that records each request and answers with the given status and body.
func flpServe(t *testing.T, status int, body string, reqs *[]flpRequest) (*app, *bytes.Buffer) {
	t.Helper()
	isolateHome(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := flpRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			rec.Fields = map[string]string{}
			for k, v := range r.MultipartForm.Value {
				rec.Fields[k] = v[0]
			}
			for k, hs := range r.MultipartForm.File {
				rec.File = k + "=" + hs[0].Filename
				f, _ := hs[0].Open()
				data, _ := io.ReadAll(f)
				f.Close()
				rec.Data = string(data)
			}
		} else {
			raw, _ := io.ReadAll(r.Body)
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &rec.Body)
			}
		}
		*reqs = append(*reqs, rec)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	var buf bytes.Buffer
	a := &app{
		printer: &output.Printer{Out: &buf},
		cfg:     &config.Config{Host: "https://beam.example.com/", Token: "tok"},
		apiCli:  client.New(ts.URL, "tok"),
	}
	return a, &buf
}

func flpRun(t *testing.T, c *cobra.Command, flags map[string]string, args ...string) error {
	t.Helper()
	for k, v := range flags {
		if err := c.Flags().Set(k, v); err != nil {
			t.Fatalf("set flag %s: %v", k, err)
		}
	}
	return c.RunE(c, args)
}

func flpWriteFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// flpChdirTemp moves into a temp dir so downloads that ignore --output never land in cmd/.
func flpChdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

func flpAPIStatus(t *testing.T, err error, want int) {
	t.Helper()
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != want {
		t.Fatalf("want APIError %d, got %v", want, err)
	}
}

func flpFields(out string, line int) []string {
	return strings.Fields(strings.Split(out, "\n")[line])
}

func TestFlpCommandMetadata(t *testing.T) {
	a := &app{}
	cases := []struct {
		cmd   *cobra.Command
		use   string
		short string
		args  int // -1 means no Args validator
	}{
		{filesCmd(a), "files", "Manage book files (EPUB, MOBI, PDF) attached to projects", -1},
		{filesListCmd(a), "list <project-id>", "List all files attached to a book project", 1},
		{filesUploadCmd(a), "upload <project-id> <file-path>", "Upload a book file (EPUB, MOBI, PDF) to a project", 2},
		{filesDownloadCmd(a), "download <project-id> <file-id>", "Download a book file to your local disk", 2},
		{filesDeleteCmd(a), "delete <project-id> <file-id>", "Delete a book file from a project", 2},
		{linksCmd(a), "links", "Manage reader signup links and landing pages", -1},
		{linksListCmd(a), "list <project-id>", "List all signup links for a book project", 1},
		{linksCreateCmd(a), "create <project-id>", "Create a new reader signup link for a project", 1},
		{linksUpdateCmd(a), "update <project-id> <link-id>", "Update an existing signup link", 2},
		{linksDeleteCmd(a), "delete <project-id> <link-id>", "Delete a signup link", 2},
		{projectsCmd(a), "projects", "Manage book projects in your catalog", -1},
		{projectsListCmd(a), "list", "List book projects for your active team", -1},
		{projectsGetCmd(a), "get <project-id>", "Get details for a single book project", 1},
		{projectsCreateCmd(a), "create", "Create a new book project", -1},
		{projectsUpdateCmd(a), "update <project-id>", "Update an existing book project", 1},
		{projectsDeleteCmd(a), "delete <project-id>", "Delete a book project and its attached files", 1},
		{projectsNewsletterCmd(a), "newsletter <project-id>", "Set project newsletter list and tags routing", 1},
	}
	for _, tc := range cases {
		t.Run(tc.use, func(t *testing.T) {
			if tc.cmd.Use != tc.use || tc.cmd.Short != tc.short {
				t.Fatalf("got Use=%q Short=%q", tc.cmd.Use, tc.cmd.Short)
			}
			if tc.args < 0 {
				return
			}
			if tc.cmd.Args == nil {
				t.Fatal("expected an Args validator")
			}
			for n := 0; n <= 3; n++ {
				err := tc.cmd.Args(tc.cmd, make([]string, n))
				if (err == nil) != (n == tc.args) {
					t.Fatalf("args=%d: got err %v", n, err)
				}
			}
		})
	}

	groups := map[string][]string{
		"file":    filesCmd(a).Aliases,
		"link":    linksCmd(a).Aliases,
		"project": projectsCmd(a).Aliases,
	}
	for want, got := range groups {
		if !reflect.DeepEqual(got, []string{want}) {
			t.Fatalf("aliases: got %v, want [%s]", got, want)
		}
	}
	names := func(c *cobra.Command) []string {
		var out []string
		for _, sub := range c.Commands() {
			out = append(out, sub.Name())
		}
		return out
	}
	if got := names(filesCmd(a)); !reflect.DeepEqual(got, []string{"delete", "download", "list", "upload"}) {
		t.Fatalf("files subcommands: %v", got)
	}
	if got := names(linksCmd(a)); !reflect.DeepEqual(got, []string{"create", "delete", "list", "update"}) {
		t.Fatalf("links subcommands: %v", got)
	}
	if got := names(projectsCmd(a)); !reflect.DeepEqual(got, []string{"create", "delete", "get", "list", "newsletter", "update"}) {
		t.Fatalf("projects subcommands: %v", got)
	}
}

func TestFlpFlagDefaults(t *testing.T) {
	a := &app{}
	for _, c := range []*cobra.Command{filesDeleteCmd(a), linksDeleteCmd(a), projectsDeleteCmd(a)} {
		f := c.Flags().Lookup("force")
		if f == nil || f.DefValue != "false" || f.Shorthand != "f" {
			t.Fatalf("%s force flag: %+v", c.Use, f)
		}
	}
	if f := filesDownloadCmd(a).Flags().Lookup("output"); f == nil || f.DefValue != "" || f.Shorthand != "o" {
		t.Fatalf("output flag: %+v", f)
	}
	if f := projectsListCmd(a).Flags().Lookup("page"); f == nil || f.DefValue != "1" {
		t.Fatalf("page flag: %+v", f)
	}
	if f := linksUpdateCmd(a).Flags().Lookup("clear-consent"); f == nil || f.DefValue != "false" {
		t.Fatalf("clear-consent flag: %+v", f)
	}
	if f := projectsUpdateCmd(a).Flags().Lookup("remove-cover"); f == nil || f.DefValue != "false" {
		t.Fatalf("remove-cover flag: %+v", f)
	}
	if f := projectsNewsletterCmd(a).Flags().Lookup("clear-tags"); f == nil || f.DefValue != "false" {
		t.Fatalf("clear-tags flag: %+v", f)
	}
}

func TestFlpFilesList(t *testing.T) {
	var reqs []flpRequest
	body := `{"data":[{"id":55,"filename":"novel.epub","file_type":"epub","file_size":1024,"downloads_count":12,"created_at":"2026-09-13"},{"id":7,"filename":"b.pdf","file_type":"pdf","file_size":99,"downloads_count":0,"created_at":"2026-09-14"}]}`
	a, buf := flpServe(t, 200, body, &reqs)
	if err := flpRun(t, filesListCmd(a), nil, "10"); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "GET" || reqs[0].Path != "/api/v1/projects/10/files" {
		t.Fatalf("requests: %+v", reqs)
	}
	out := buf.String()
	if got := flpFields(out, 0); !reflect.DeepEqual(got, []string{"ID", "FILENAME", "FORMAT", "SIZE", "(BYTES)", "DOWNLOADS", "CREATED"}) {
		t.Fatalf("header: %v", got)
	}
	if got := flpFields(out, 1); !reflect.DeepEqual(got, []string{"55", "novel.epub", "EPUB", "1024", "12", "2026-09-13"}) {
		t.Fatalf("row1: %v", got)
	}
	if got := flpFields(out, 2); !reflect.DeepEqual(got, []string{"7", "b.pdf", "PDF", "99", "0", "2026-09-14"}) {
		t.Fatalf("row2: %v", got)
	}

	a, buf = flpServe(t, 200, `{"data":[]}`, &reqs)
	if err := flpRun(t, filesListCmd(a), nil, "10"); err != nil || buf.String() != "No files attached to this project.\n" {
		t.Fatalf("empty: %v %q", err, buf.String())
	}

	a, buf = flpServe(t, 200, `{"data":[{"id":1}]}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, filesListCmd(a), nil, "10"); err != nil || buf.String() != "{\n  \"data\": [\n    {\n      \"id\": 1\n    }\n  ]\n}\n" {
		t.Fatalf("json: %v %q", err, buf.String())
	}

	a, buf = flpServe(t, 200, `{"data":"nope"}`, &reqs)
	if err := flpRun(t, filesListCmd(a), nil, "10"); err == nil || buf.Len() != 0 {
		t.Fatalf("bad json: %v %q", err, buf.String())
	}

	a, _ = flpServe(t, 404, `{"message":"missing"}`, &reqs)
	flpAPIStatus(t, flpRun(t, filesListCmd(a), nil, "10"), 404)
}

func TestFlpFilesUpload(t *testing.T) {
	for _, name := range []string{"book.epub", "book.mobi", "BOOK.PDF"} {
		t.Run(name, func(t *testing.T) {
			var reqs []flpRequest
			a, buf := flpServe(t, 201, `{"data":{"id":88,"filename":"stored.bin"}}`, &reqs)
			p := flpWriteFile(t, name, "twelve bytes")
			if err := flpRun(t, filesUploadCmd(a), nil, "3", p); err != nil {
				t.Fatal(err)
			}
			want := "Uploading " + name + " (12 bytes) to project #3...\n✓ Uploaded file #88 (stored.bin) successfully.\n"
			if buf.String() != want {
				t.Fatalf("got %q", buf.String())
			}
			if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/api/v1/projects/3/files" ||
				reqs[0].File != "file="+name || reqs[0].Data != "twelve bytes" || len(reqs[0].Fields) != 0 {
				t.Fatalf("requests: %+v", reqs)
			}
		})
	}

	var reqs []flpRequest
	a, buf := flpServe(t, 201, `{"data":{"id":1}}`, &reqs)
	err := flpRun(t, filesUploadCmd(a), nil, "3", filepath.Join(t.TempDir(), "absent.epub"))
	if err == nil || !errors.Is(err, fs.ErrNotExist) || !strings.HasPrefix(err.Error(), "file not found: ") {
		t.Fatalf("missing file: %v", err)
	}

	err = flpRun(t, filesUploadCmd(a), nil, "3", flpWriteFile(t, "notes.txt", "x"))
	if err == nil || err.Error() != "unsupported file extension '.txt'; allowed extensions are .epub, .mobi, .pdf" {
		t.Fatalf("ext: %v", err)
	}
	err = flpRun(t, filesUploadCmd(a), nil, "3", flpWriteFile(t, "noext", "x"))
	if err == nil || err.Error() != "unsupported file extension ''; allowed extensions are .epub, .mobi, .pdf" {
		t.Fatalf("no ext: %v", err)
	}
	if len(reqs) != 0 || buf.Len() != 0 {
		t.Fatalf("expected no request/output: %+v %q", reqs, buf.String())
	}

	a, buf = flpServe(t, 201, `{"data":{"id":1}}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, filesUploadCmd(a), nil, "3", flpWriteFile(t, "a.epub", "x")); err != nil || buf.String() != "{\n  \"data\": {\n    \"id\": 1\n  }\n}\n" {
		t.Fatalf("json: %v %q", err, buf.String())
	}

	a, buf = flpServe(t, 201, `[]`, &reqs)
	a.printer.Quiet = true
	if err := flpRun(t, filesUploadCmd(a), nil, "3", flpWriteFile(t, "a.epub", "x")); err == nil {
		t.Fatal("expected unmarshal error")
	}

	a, _ = flpServe(t, 422, `{"message":"too big"}`, &reqs)
	flpAPIStatus(t, flpRun(t, filesUploadCmd(a), nil, "3", flpWriteFile(t, "a.epub", "x")), 422)
}

func TestFlpFilesDownload(t *testing.T) {
	isolateHome(t)
	var gotPath string
	disposition := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		if disposition != "" {
			w.Header().Set("Content-Disposition", disposition)
		}
		_, _ = w.Write([]byte("book-bytes"))
	}))
	t.Cleanup(ts.Close)
	var buf bytes.Buffer
	a := &app{printer: &output.Printer{Out: &buf}, cfg: &config.Config{}, apiCli: client.New(ts.URL, "tok")}

	dir := flpChdirTemp(t)
	var err error

	cases := []struct {
		name, flag, disposition, dest string
	}{
		{"flag", filepath.Join(dir, "chosen.epub"), `attachment; filename="server.epub"`, filepath.Join(dir, "chosen.epub")},
		{"header", "", `attachment; filename="../evil/server.epub"`, "server.epub"},
		{"fallback", "", "", "project-4-file-9"},
		{"unparseable", "", "attachment; filename=", "project-4-file-9"},
		{"no filename", "", "attachment", "project-4-file-9"},
		{"blank filename", "", `attachment; filename="a/ /"`, "project-4-file-9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			disposition = tc.disposition
			flags := map[string]string{}
			if tc.flag != "" {
				flags["output"] = tc.flag
			}
			if err := flpRun(t, filesDownloadCmd(a), flags, "4", "9"); err != nil {
				t.Fatal(err)
			}
			if gotPath != "GET /api/v1/projects/4/files/9/download" {
				t.Fatalf("request: %s", gotPath)
			}
			want := "Downloading file to " + tc.dest + "...\n✓ Download complete (10 bytes written to " + tc.dest + ").\n"
			if buf.String() != want {
				t.Fatalf("got %q", buf.String())
			}
			data, err := os.ReadFile(filepath.Join(dir, filepath.Base(tc.dest)))
			if err != nil || string(data) != "book-bytes" {
				t.Fatalf("file: %v %q", err, data)
			}
			_ = os.Remove(filepath.Join(dir, filepath.Base(tc.dest)))
		})
	}

	buf.Reset()
	disposition = ""
	err = flpRun(t, filesDownloadCmd(a), map[string]string{"output": filepath.Join(dir, "no", "such", "x")}, "4", "9")
	if err == nil || !errors.Is(err, fs.ErrNotExist) || !strings.HasPrefix(err.Error(), "failed to create destination file: ") {
		t.Fatalf("create: %v", err)
	}
	if strings.Contains(buf.String(), "complete") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}

func TestFlpFilesDownloadErrors(t *testing.T) {
	isolateHome(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/files/404/") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("short"))
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	t.Cleanup(ts.Close)
	var buf bytes.Buffer
	a := &app{printer: &output.Printer{Out: &buf}, cfg: &config.Config{}, apiCli: client.New(ts.URL, "tok")}
	dest := filepath.Join(flpChdirTemp(t), "partial.epub")

	err := flpRun(t, filesDownloadCmd(a), map[string]string{"output": dest}, "4", "9")
	if err == nil || !strings.HasPrefix(err.Error(), "failed to write file contents: ") || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("copy: %v", err)
	}
	if buf.String() != "Downloading file to "+dest+"...\n" {
		t.Fatalf("got %q", buf.String())
	}

	buf.Reset()
	flpAPIStatus(t, flpRun(t, filesDownloadCmd(a), map[string]string{"output": dest}, "4", "404"), 404)
	if buf.Len() != 0 {
		t.Fatalf("got %q", buf.String())
	}
}

func TestFlpSanitizeFilenameWhitespaceBase(t *testing.T) {
	if got := sanitizeFilename("a/ /"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := extractDispositionFilename("attachment; filename"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := extractDispositionFilename("attachment; name=x"); got != "" {
		t.Fatalf("got %q", got)
	}
}

type flpDeleteCase struct {
	name    string
	build   func(*app) *cobra.Command
	args    []string
	path    string
	prompt  string
	success string
}

func flpDeleteCases() []flpDeleteCase {
	return []flpDeleteCase{
		{"files", filesDeleteCmd, []string{"5", "6"}, "/api/v1/projects/5/files/6",
			"Are you sure you want to delete file #6 from project #5? (y/N): ", "✓ Deleted file #6 from project #5\n"},
		{"links", linksDeleteCmd, []string{"5", "6"}, "/api/v1/projects/5/links/6",
			"Are you sure you want to delete link #6? (y/N): ", "✓ Deleted signup link #6\n"},
		{"projects", projectsDeleteCmd, []string{"5"}, "/api/v1/projects/5",
			"Are you sure you want to delete project #5? (y/N): ", "✓ Deleted book project #5\n"},
	}
}

func TestFlpDeleteCommands(t *testing.T) {
	for _, dc := range flpDeleteCases() {
		t.Run(dc.name, func(t *testing.T) {
			answers := []struct {
				in      string
				deleted bool
			}{{"y\n", true}, {" YES \n", true}, {"n\n", false}, {"", false}, {"yep\n", false}}
			for _, ans := range answers {
				var reqs []flpRequest
				a, buf := flpServe(t, 200, `{"message":"ok"}`, &reqs)
				c := dc.build(a)
				var prompt bytes.Buffer
				c.SetIn(strings.NewReader(ans.in))
				c.SetOut(&prompt)
				if err := flpRun(t, c, nil, dc.args...); err != nil {
					t.Fatal(err)
				}
				if prompt.String() != dc.prompt {
					t.Fatalf("prompt %q", prompt.String())
				}
				if ans.deleted {
					if len(reqs) != 1 || reqs[0].Method != "DELETE" || reqs[0].Path != dc.path || buf.String() != dc.success {
						t.Fatalf("answer %q: %+v %q", ans.in, reqs, buf.String())
					}
				} else if len(reqs) != 0 || buf.String() != "Cancelled.\n" {
					t.Fatalf("answer %q: %+v %q", ans.in, reqs, buf.String())
				}
			}

			var reqs []flpRequest
			a, buf := flpServe(t, 200, `{"message":"ok"}`, &reqs)
			c := dc.build(a)
			var prompt bytes.Buffer
			c.SetOut(&prompt)
			if err := flpRun(t, c, map[string]string{"force": "true"}, dc.args...); err != nil {
				t.Fatal(err)
			}
			if prompt.Len() != 0 || len(reqs) != 1 || buf.String() != dc.success {
				t.Fatalf("force: %q %+v %q", prompt.String(), reqs, buf.String())
			}

			a, buf = flpServe(t, 200, `{"message":"ok"}`, &reqs)
			a.printer.JSON = true
			c = dc.build(a)
			prompt.Reset()
			c.SetOut(&prompt)
			c.SetIn(strings.NewReader(""))
			if err := flpRun(t, c, nil, dc.args...); err != nil {
				t.Fatal(err)
			}
			if prompt.Len() != 0 || buf.String() != "{\n  \"message\": \"ok\"\n}\n" {
				t.Fatalf("json: %q %q", prompt.String(), buf.String())
			}

			a, buf = flpServe(t, 403, `{"message":"no"}`, &reqs)
			flpAPIStatus(t, flpRun(t, dc.build(a), map[string]string{"force": "true"}, dc.args...), 403)
			if buf.Len() != 0 {
				t.Fatalf("error output %q", buf.String())
			}
		})
	}
}

func TestFlpLinksList(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 200, `{"data":[{"id":3,"title":"Promo","slug":"abc","created_at":"2026-01-02"}]}`, &reqs)
	if err := flpRun(t, linksListCmd(a), nil, "8"); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "GET" || reqs[0].Path != "/api/v1/projects/8/links" {
		t.Fatalf("requests %+v", reqs)
	}
	if got := flpFields(buf.String(), 0); !reflect.DeepEqual(got, []string{"ID", "TITLE", "SLUG", "PUBLIC", "URL", "CREATED"}) {
		t.Fatalf("header %v", got)
	}
	if got := flpFields(buf.String(), 1); !reflect.DeepEqual(got, []string{"3", "Promo", "abc", "https://beam.example.com/download/abc", "2026-01-02"}) {
		t.Fatalf("row %v", got)
	}

	a, buf = flpServe(t, 200, `{"data":[]}`, &reqs)
	if err := flpRun(t, linksListCmd(a), nil, "8"); err != nil || buf.String() != "No signup links found for this project.\n" {
		t.Fatalf("empty %v %q", err, buf.String())
	}
	a, buf = flpServe(t, 200, `{"data":[]}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, linksListCmd(a), nil, "8"); err != nil || buf.String() != "{\n  \"data\": []\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, buf = flpServe(t, 200, `{"data":{}}`, &reqs)
	if err := flpRun(t, linksListCmd(a), nil, "8"); err == nil || buf.Len() != 0 {
		t.Fatalf("bad json %v", err)
	}
	a, _ = flpServe(t, 500, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, linksListCmd(a), nil, "8"), 500)
}

func TestFlpLinksCreate(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 201, `{"data":{"id":12,"slug":"xyz"}}`, &reqs)
	if err := flpRun(t, linksCreateCmd(a), map[string]string{"title": "T", "consent": "C"}, "8"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "✓ Created signup link #12: https://beam.example.com/download/xyz\n" {
		t.Fatalf("got %q", buf.String())
	}
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/api/v1/projects/8/links" ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "T", "opt_in_text": "C"}) {
		t.Fatalf("requests %+v", reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 201, `{"data":{"id":1}}`, &reqs)
	if err := flpRun(t, linksCreateCmd(a), nil, "8"); err != nil || len(reqs) != 1 || len(reqs[0].Body) != 0 {
		t.Fatalf("empty payload %v %+v", err, reqs)
	}

	a, buf = flpServe(t, 201, `{"data":{"id":1}}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, linksCreateCmd(a), nil, "8"); err != nil || buf.String() != "{\n  \"data\": {\n    \"id\": 1\n  }\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 201, `{"data":[]}`, &reqs)
	if err := flpRun(t, linksCreateCmd(a), nil, "8"); err == nil {
		t.Fatal("expected unmarshal error")
	}
	a, _ = flpServe(t, 422, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, linksCreateCmd(a), nil, "8"), 422)
}

func TestFlpLinksUpdate(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 200, `{"data":{"id":6}}`, &reqs)
	if err := flpRun(t, linksUpdateCmd(a), map[string]string{"title": "New", "consent": "Agree"}, "8", "6"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "✓ Updated signup link #6\n" {
		t.Fatalf("got %q", buf.String())
	}
	if len(reqs) != 1 || reqs[0].Method != "PUT" || reqs[0].Path != "/api/v1/projects/8/links/6" ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "New", "opt_in_text": "Agree"}) {
		t.Fatalf("requests %+v", reqs)
	}

	reqs = nil
	a, buf = flpServe(t, 200, `{"data":{"id":6}}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, linksUpdateCmd(a), map[string]string{"title": "New", "clear-consent": "true"}, "8", "6"); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "PUT" || !reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "New", "opt_in_text": nil}) {
		t.Fatalf("clear without fetch: %+v", reqs)
	}
	if buf.String() != "{\n  \"data\": {\n    \"id\": 6\n  }\n}\n" {
		t.Fatalf("json %q", buf.String())
	}

	reqs = nil
	a, buf = flpServe(t, 200, `{}`, &reqs)
	err := flpRun(t, linksUpdateCmd(a), map[string]string{"consent": "C", "clear-consent": "true"}, "8", "6")
	if err == nil || err.Error() != "cannot specify both --consent and --clear-consent" || len(reqs) != 0 {
		t.Fatalf("conflict %v %+v", err, reqs)
	}

	a, buf = flpServe(t, 200, `{"data":[]}`, &reqs)
	err = flpRun(t, linksUpdateCmd(a), map[string]string{"title": "X"}, "8", "6")
	if err == nil || err.Error() != "signup link #6 not found in project 8" || buf.Len() != 0 {
		t.Fatalf("not found %v", err)
	}

	a, _ = flpServe(t, 409, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, linksUpdateCmd(a), map[string]string{"title": "X", "consent": "Y"}, "8", "6"), 409)
}

func TestFlpLinkUpdatePayloadSkipsFetchWhenClearingWithTitle(t *testing.T) {
	calls := 0
	fetch := func() (*SignupLinkItem, error) {
		calls++
		return &SignupLinkItem{Title: "Old", OptInText: "Old consent"}, nil
	}
	got, err := linkUpdatePayload("New", "", true, fetch)
	if err != nil || calls != 0 || !reflect.DeepEqual(got, map[string]any{"title": "New", "opt_in_text": nil}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	got, err = linkUpdatePayload("", "", true, fetch)
	if err != nil || calls != 1 || !reflect.DeepEqual(got, map[string]any{"title": "Old", "opt_in_text": nil}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
}

func TestFlpProjectsList(t *testing.T) {
	var reqs []flpRequest
	body := `{"data":[{"id":2,"title":"Saga","files_count":3,"signup_links_count":4,"created_at":"2026-02-03"}],"current_page":2,"last_page":5,"total":41}`
	a, buf := flpServe(t, 200, body, &reqs)
	if err := flpRun(t, projectsListCmd(a), map[string]string{"page": "2"}); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "GET" || reqs[0].Path != "/api/v1/projects" || reqs[0].Query != "page=2" {
		t.Fatalf("requests %+v", reqs)
	}
	out := buf.String()
	if got := flpFields(out, 0); !reflect.DeepEqual(got, []string{"ID", "TITLE", "FILES", "LINKS", "CREATED"}) {
		t.Fatalf("header %v", got)
	}
	if got := flpFields(out, 1); !reflect.DeepEqual(got, []string{"2", "Saga", "3", "4", "2026-02-03"}) {
		t.Fatalf("row %v", got)
	}
	if !strings.HasSuffix(out, "\n\nPage 2 of 5 (Total: 41)\n") {
		t.Fatalf("footer %q", out)
	}

	reqs = nil
	a, buf = flpServe(t, 200, `{"data":[]}`, &reqs)
	if err := flpRun(t, projectsListCmd(a), nil); err != nil || buf.String() != "No book projects found. Create one with 'bookbeam projects create'.\n" {
		t.Fatalf("empty %v %q", err, buf.String())
	}
	if reqs[0].Query != "page=1" {
		t.Fatalf("default page query %q", reqs[0].Query)
	}

	reqs = nil
	a, _ = flpServe(t, 200, `{"data":[]}`, &reqs)
	if err := flpRun(t, projectsListCmd(a), map[string]string{"page": "0"}); err != nil || reqs[0].Query != "" {
		t.Fatalf("page 0 %v %+v", err, reqs)
	}

	a, buf = flpServe(t, 200, `{"total":1}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, projectsListCmd(a), nil); err != nil || buf.String() != "{\n  \"total\": 1\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, buf = flpServe(t, 200, `{"data":[{"id":1}],"total":"x"}`, &reqs)
	if err := flpRun(t, projectsListCmd(a), nil); err == nil || buf.Len() != 0 {
		t.Fatalf("bad json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 401, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, projectsListCmd(a), nil), 401)
}

func TestFlpProjectsGet(t *testing.T) {
	var reqs []flpRequest
	body := `{"data":{"id":9,"title":"Saga","description":"Space","cover_image_url":"https://c.example.com/x.png","files_count":1,"signup_links_count":2,"created_at":"2026-03-04"}}`
	a, buf := flpServe(t, 200, body, &reqs)
	if err := flpRun(t, projectsGetCmd(a), nil, "9"); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "GET" || reqs[0].Path != "/api/v1/projects/9" {
		t.Fatalf("requests %+v", reqs)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	want := [][]string{
		{"Field", "Value"}, {"ID", "9"}, {"Title", "Saga"}, {"Description", "Space"},
		{"Cover", "URL", "https://c.example.com/x.png"}, {"Files", "Count", "1"},
		{"Links", "Count", "2"}, {"Created", "At", "2026-03-04"},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines %q", lines)
	}
	for i, w := range want {
		if got := strings.Fields(lines[i]); !reflect.DeepEqual(got, w) {
			t.Fatalf("line %d: %v", i, got)
		}
	}

	a, buf = flpServe(t, 200, `{"data":{"id":9}}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, projectsGetCmd(a), nil, "9"); err != nil || buf.String() != "{\n  \"data\": {\n    \"id\": 9\n  }\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 200, `{"data":1}`, &reqs)
	if err := flpRun(t, projectsGetCmd(a), nil, "9"); err == nil {
		t.Fatal("expected unmarshal error")
	}
	a, _ = flpServe(t, 404, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, projectsGetCmd(a), nil, "9"), 404)
}

func TestFlpProjectsCreate(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 201, `{"data":{"id":31,"title":"Stored"}}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T", "description": "D"}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "✓ Created book project #31: Stored\n" {
		t.Fatalf("got %q", buf.String())
	}
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/api/v1/projects" ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "T", "description": "D"}) {
		t.Fatalf("requests %+v", reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 201, `{"data":{"id":31}}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T"}); err != nil ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "T"}) {
		t.Fatalf("title only %v %+v", err, reqs)
	}

	cover := flpWriteFile(t, "cover.png", "png")
	reqs = nil
	a, buf = flpServe(t, 201, `{"data":{"id":32,"title":"C"}}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T", "description": "D", "cover": cover}); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/api/v1/projects" || reqs[0].File != "cover_image=cover.png" ||
		!reflect.DeepEqual(reqs[0].Fields, map[string]string{"title": "T", "description": "D"}) || buf.String() != "✓ Created book project #32: C\n" {
		t.Fatalf("cover %+v %q", reqs, buf.String())
	}
	reqs = nil
	a, _ = flpServe(t, 201, `{"data":{"id":32}}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T", "cover": cover}); err != nil ||
		!reflect.DeepEqual(reqs[0].Fields, map[string]string{"title": "T"}) {
		t.Fatalf("cover title only %v %+v", err, reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 201, `{}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"description": "D"}); err == nil || err.Error() != "--title is required" || len(reqs) != 0 {
		t.Fatalf("required %v", err)
	}

	a, buf = flpServe(t, 201, `{"data":{"id":1}}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T"}); err != nil || buf.String() != "{\n  \"data\": {\n    \"id\": 1\n  }\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 201, `{"data":"x"}`, &reqs)
	if err := flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T"}); err == nil {
		t.Fatal("expected unmarshal error")
	}
	a, _ = flpServe(t, 422, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, projectsCreateCmd(a), map[string]string{"title": "T", "cover": cover}), 422)
}

func TestFlpProjectsUpdate(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 200, `{"data":{}}`, &reqs)
	if err := flpRun(t, projectsUpdateCmd(a), map[string]string{"title": "T", "description": "D", "remove-cover": "true"}, "4"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "✓ Updated book project #4\n" {
		t.Fatalf("got %q", buf.String())
	}
	if len(reqs) != 1 || reqs[0].Method != "PUT" || reqs[0].Path != "/api/v1/projects/4" ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"title": "T", "description": "D", "remove_cover_image": true}) {
		t.Fatalf("requests %+v", reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 200, `{}`, &reqs)
	if err := flpRun(t, projectsUpdateCmd(a), nil, "4"); err != nil || len(reqs[0].Body) != 0 {
		t.Fatalf("empty payload %v %+v", err, reqs)
	}

	cover := flpWriteFile(t, "new.jpg", "jpg")
	reqs = nil
	a, buf = flpServe(t, 200, `{}`, &reqs)
	if err := flpRun(t, projectsUpdateCmd(a), map[string]string{"title": "T", "cover": cover}, "4"); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/api/v1/projects/4" || reqs[0].File != "cover_image=new.jpg" ||
		!reflect.DeepEqual(reqs[0].Fields, map[string]string{"title": "T", "_method": "PUT"}) || buf.String() != "✓ Updated book project #4\n" {
		t.Fatalf("cover %+v %q", reqs, buf.String())
	}

	reqs = nil
	a, _ = flpServe(t, 200, `{}`, &reqs)
	err := flpRun(t, projectsUpdateCmd(a), map[string]string{"cover": cover, "remove-cover": "true"}, "4")
	if err == nil || err.Error() != "cannot specify both --cover and --remove-cover" || len(reqs) != 0 {
		t.Fatalf("conflict %v", err)
	}

	a, buf = flpServe(t, 200, `{"ok":true}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, projectsUpdateCmd(a), nil, "4"); err != nil || buf.String() != "{\n  \"ok\": true\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 403, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, projectsUpdateCmd(a), nil, "4"), 403)
}

func TestFlpBuildProjectUpdateMultipartFields(t *testing.T) {
	if got := buildProjectUpdateMultipartFields("", "", false); !reflect.DeepEqual(got, map[string]string{"_method": "PUT"}) {
		t.Fatalf("got %v", got)
	}
	want := map[string]string{"title": "A", "description": "B", "remove_cover_image": "true", "_method": "PUT"}
	if got := buildProjectUpdateMultipartFields("A", "B", true); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestFlpProjectsNewsletter(t *testing.T) {
	var reqs []flpRequest
	a, buf := flpServe(t, 200, `{}`, &reqs)
	if err := flpRun(t, projectsNewsletterCmd(a), map[string]string{"list-id": "L1", "tags": "a,b"}, "4"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "✓ Updated newsletter routing for project #4\n" {
		t.Fatalf("got %q", buf.String())
	}
	if len(reqs) != 1 || reqs[0].Method != "PUT" || reqs[0].Path != "/api/v1/projects/4/newsletter" ||
		!reflect.DeepEqual(reqs[0].Body, map[string]any{"newsletter_list_id": "L1", "newsletter_tags": "a,b"}) {
		t.Fatalf("requests %+v", reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 200, `{}`, &reqs)
	err := flpRun(t, projectsNewsletterCmd(a), map[string]string{"tags": "a", "clear-tags": "true"}, "4")
	if err == nil || err.Error() != "cannot specify both --tags and --clear-tags" || len(reqs) != 0 {
		t.Fatalf("conflict %v %+v", err, reqs)
	}

	reqs = nil
	a, _ = flpServe(t, 200, `{}`, &reqs)
	if err := flpRun(t, projectsNewsletterCmd(a), nil, "4"); err == nil || err.Error() != "specify --list-id, --tags, or --clear-tags" || len(reqs) != 0 {
		t.Fatalf("required %v", err)
	}

	a, buf = flpServe(t, 200, `{"ok":1}`, &reqs)
	a.printer.JSON = true
	if err := flpRun(t, projectsNewsletterCmd(a), map[string]string{"list-id": "L1", "tags": "a"}, "4"); err != nil || buf.String() != "{\n  \"ok\": 1\n}\n" {
		t.Fatalf("json %v %q", err, buf.String())
	}
	a, _ = flpServe(t, 422, `{}`, &reqs)
	flpAPIStatus(t, flpRun(t, projectsNewsletterCmd(a), map[string]string{"list-id": "L1", "tags": "a"}, "4"), 422)
}

func TestFlpProjectNewsletterPayloadSkipsFetchWhenComplete(t *testing.T) {
	calls := 0
	fetch := func() (*ProjectItem, error) {
		calls++
		return &ProjectItem{NewsletterListID: "old-list", NewsletterTags: "old-tags"}, nil
	}
	got, err := projectNewsletterPayload("L1", "a,b", false, fetch)
	if err != nil || calls != 0 || !reflect.DeepEqual(got, map[string]any{"newsletter_list_id": "L1", "newsletter_tags": "a,b"}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	got, err = projectNewsletterPayload("L1", "", true, fetch)
	if err != nil || calls != 0 || !reflect.DeepEqual(got, map[string]any{"newsletter_list_id": "L1", "newsletter_tags": nil}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	got, err = projectNewsletterPayload("", "", true, fetch)
	if err != nil || calls != 1 || !reflect.DeepEqual(got, map[string]any{"newsletter_list_id": "old-list", "newsletter_tags": nil}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	got, err = projectNewsletterPayload("L1", "", false, fetch)
	if err != nil || calls != 2 || !reflect.DeepEqual(got, map[string]any{"newsletter_list_id": "L1", "newsletter_tags": "old-tags"}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	got, err = projectNewsletterPayload("", "new", false, fetch)
	if err != nil || calls != 3 || !reflect.DeepEqual(got, map[string]any{"newsletter_list_id": "old-list", "newsletter_tags": "new"}) {
		t.Fatalf("got %v %v calls=%d", got, err, calls)
	}
	fetchErr := errors.New("fetch failed")
	got, err = projectNewsletterPayload("", "new", false, func() (*ProjectItem, error) { return nil, fetchErr })
	if err != fetchErr || got != nil {
		t.Fatalf("got %v %v", got, err)
	}
}
