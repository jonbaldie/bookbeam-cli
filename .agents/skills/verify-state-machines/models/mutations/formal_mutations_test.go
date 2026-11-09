package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

func formalMutationRun(command *cobra.Command, args []string) error {
	command.SetContext(context.Background())
	return command.RunE(command, args)
}

func TestFormalFieldEditGeneratedInputs(t *testing.T) {
	for _, value := range []string{"", "x", "a,b", "雪", "\x00"} {
		for _, clear := range []bool{false, true} {
			command := &cobra.Command{}
			command.Flags().String("value", value, "")
			command.Flags().Bool("clear", clear, "")
			edit, err := editFlag(command, "value", "clear")
			if value != "" && clear {
				if err == nil {
					t.Fatalf("accepted conflicting value %q and clear", value)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			jsonBody := map[string]any{}
			formBody := map[string]string{}
			edit.putJSON(jsonBody, "field")
			edit.putForm(formBody, "field")
			j, jok := jsonBody["field"]
			f, fok := formBody["field"]
			switch {
			case clear:
				if !jok || j != nil || !fok || f != "" {
					t.Fatalf("clear semantics %v %v", jsonBody, formBody)
				}
			case value != "":
				if !jok || j != value || !fok || f != value {
					t.Fatalf("set semantics %v %v", jsonBody, formBody)
				}
			default:
				if jok || fok {
					t.Fatalf("unset emitted %v %v", jsonBody, formBody)
				}
			}
			for _, existing := range []string{"", "old", "雪"} {
				merged := edit.orExisting(existing)
				want := value
				if edit.unset() {
					want = existing
				}
				if merged.value != want || merged.clear != clear {
					t.Fatalf("merge changed requested edit %+v", merged)
				}
			}
		}
	}
}

func TestFormalReadMergeGuards(t *testing.T) {
	for _, missing := range []bool{false, true} {
		calls := 0
		title := "new"
		consent := fieldEdit{value: "new"}
		listID := "list"
		tags := fieldEdit{clear: true}
		if missing {
			title = ""
			listID = ""
		}
		_, linkErr := linkUpdatePayload(title, consent, func() (*SignupLinkItem, error) { calls++; return nil, fmt.Errorf("read failed") })
		_, newsletterErr := projectNewsletterPayload(listID, tags, func() (*ProjectItem, error) { calls++; return nil, fmt.Errorf("read failed") })
		if missing && (calls != 2 || linkErr == nil || newsletterErr == nil) {
			t.Fatal("failed read did not prevent payload")
		}
		if !missing && (calls != 0 || linkErr != nil || newsletterErr != nil) {
			t.Fatal("complete edit unexpectedly read")
		}
	}
}

func TestFormalStaleLinkOverwriteReplay(t *testing.T) {
	remoteTitle := "old"
	gets := 0
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			gets++
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": 2, "title": remoteTitle, "opt_in_text": "old consent"}}})
			remoteTitle = "concurrent title"
		case http.MethodPut:
			puts++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			remoteTitle, _ = payload["title"].(string)
			_, _ = io.WriteString(w, `{"data":{"id":2}}`)
		}
	}))
	defer server.Close()
	a := &app{apiCli: client.New(server.URL, "token"), printer: &output.Printer{Out: io.Discard, JSON: true}}
	command := linksUpdateCmd(a)
	if err := command.Flags().Set("consent", "new consent"); err != nil {
		t.Fatal(err)
	}
	if err := formalMutationRun(command, []string{"1", "2"}); err != nil {
		t.Fatal(err)
	}
	if gets != 1 || puts != 1 || remoteTitle != "old" {
		t.Fatalf("counterexample changed gets=%d puts=%d remote=%q", gets, puts, remoteTitle)
	}
}

func TestFormalStaleNewsletterOverwriteReplay(t *testing.T) {
	remoteList := "old-list"
	gets := 0
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": 1, "newsletter_list_id": remoteList, "newsletter_tags": []string{"old"}}})
			remoteList = "concurrent-list"
			return
		}
		puts++
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		remoteList, _ = payload["newsletter_list_id"].(string)
		_, _ = io.WriteString(w, `{"data":{"id":1}}`)
	}))
	defer server.Close()
	a := &app{apiCli: client.New(server.URL, "token"), printer: &output.Printer{Out: io.Discard, JSON: true}}
	command := projectsNewsletterCmd(a)
	_ = command.Flags().Set("tags", "new")
	if err := formalMutationRun(command, []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if gets != 1 || puts != 1 || remoteList != "old-list" {
		t.Fatalf("counterexample changed gets=%d puts=%d remote=%q", gets, puts, remoteList)
	}
}

func TestFormalDeleteAuthorizationGeneratedInputs(t *testing.T) {
	for _, resource := range []string{"project", "file", "link"} {
		for _, valid := range []bool{false, true} {
			for _, force := range []bool{false, true} {
				for _, asJSON := range []bool{false, true} {
					for _, answer := range []string{"", "no", " y ", "YES"} {
						deletes := 0
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.Method == http.MethodDelete {
								deletes++
							}
							_, _ = io.WriteString(w, `{}`)
						}))
						var out bytes.Buffer
						a := &app{apiCli: client.New(server.URL, "token"), printer: &output.Printer{Out: &out, In: strings.NewReader(answer), JSON: asJSON}}
						var command *cobra.Command
						args := []string{"1"}
						switch resource {
						case "project":
							command = projectsDeleteCmd(a)
						case "file":
							command = filesDeleteCmd(a)
							args = []string{"1", "2"}
						case "link":
							command = linksDeleteCmd(a)
							args = []string{"1", "2"}
						}
						if !valid {
							args[0] = "bad"
						}
						if force {
							_ = command.Flags().Set("force", "true")
						}
						err := formalMutationRun(command, args)
						expected := valid && (force || asJSON || strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes"))
						if (deletes == 1) != expected {
							t.Fatalf("%s valid=%v force=%v json=%v answer=%q deletes=%d", resource, valid, force, asJSON, answer, deletes)
						}
						if valid && err != nil {
							t.Fatal(err)
						}
						if !valid && err == nil {
							t.Fatal("invalid id accepted")
						}
						server.Close()
					}
				}
			}
		}
	}
}

func TestFormalRemoteCreateThenDecodeFailureReplay(t *testing.T) {
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { created++; _, _ = io.WriteString(w, `not json`) }))
	defer server.Close()
	a := &app{apiCli: client.New(server.URL, "token"), printer: &output.Printer{Out: io.Discard, JSON: true}}
	command := projectsCreateCmd(a)
	_ = command.Flags().Set("title", "Book")
	if err := formalMutationRun(command, nil); err == nil {
		t.Fatal("malformed response unexpectedly succeeded")
	}
	if created != 1 {
		t.Fatalf("expected one creation request before local error, got %d", created)
	}
}

func TestFormalRemoteProjectionBoundaries(t *testing.T) {
	var paths []string
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer server.Close()
	a := &app{apiCli: client.New(server.URL, "token"), printer: &output.Printer{Out: io.Discard, JSON: true}}
	configure := newsletterConfigureCmd(a)
	if err := formalMutationRun(configure, nil); err == nil || len(paths) != 0 {
		t.Fatal("missing provider did not prevent request")
	}
	_ = configure.Flags().Set("provider", "CUSTOM")
	_ = configure.Flags().Set("api-key", "secret")
	if err := formalMutationRun(configure, nil); err != nil {
		t.Fatal(err)
	}
	webhook := newsletterWebhookCmd(a)
	if err := formalMutationRun(webhook, nil); err == nil || len(paths) != 1 {
		t.Fatal("omitted webhook URL did not prevent request")
	}
	_ = webhook.Flags().Set("url", "")
	if err := formalMutationRun(webhook, nil); err != nil {
		t.Fatal(err)
	}
	webhookSet := newsletterWebhookCmd(a)
	_ = webhookSet.Flags().Set("url", "https://example.test/hooks")
	if err := formalMutationRun(webhookSet, nil); err != nil {
		t.Fatal(err)
	}
	lists := newsletterListsCmd(a)
	if err := formalMutationRun(lists, nil); err != nil {
		t.Fatal(err)
	}
	disconnect := newsletterDisconnectCmd(a)
	if err := formalMutationRun(disconnect, nil); err != nil {
		t.Fatal(err)
	}
	if bodies[0]["provider"] != "custom" || bodies[1]["webhook_url"] != "" || bodies[2]["webhook_url"] != "https://example.test/hooks" || !strings.HasPrefix(paths[3], "POST ") || !strings.HasPrefix(paths[4], "DELETE ") {
		t.Fatalf("projection requests changed %v %v", paths, bodies)
	}
}

func TestFormalProjectionGeneratedInputs(t *testing.T) {
	for _, provider := range []string{"", "kit"} {
		for _, token := range []string{"", "secret"} {
			for _, webhook := range []string{"", "https://example.test"} {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"provider": provider, "config": map[string]string{"api_token": token}, "webhook_url": webhook})
				}))
				var out bytes.Buffer
				a := &app{apiCli: client.New(server.URL, ""), printer: &output.Printer{Out: &out}}
				c := newsletterStatusCmd(a)
				if err := formalMutationRun(c, nil); err != nil {
					t.Fatal(err)
				}
				want := "No"
				if provider != "" && token != "" {
					want = "Yes"
				}
				if !strings.Contains(out.String(), "Configured") {
					t.Fatalf("missing Configured row: %q", out.String())
				}
				for _, line := range strings.Split(out.String(), "\n") {
					if strings.HasPrefix(line, "Configured") && !strings.HasSuffix(strings.TrimSpace(line), want) {
						t.Fatalf("provider=%q token=%q webhook=%q output=%q", provider, token, webhook, out.String())
					}
				}
				out.Reset()
				a.printer.JSON = true
				if err := formalMutationRun(c, nil); err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
					t.Fatal(err)
				}
				if doc["configured"] != (provider != "" && token != "") || doc["provider"] != provider || doc["webhook_url"] != webhook {
					t.Fatalf("derived JSON changed provider=%q token=%q webhook=%q output=%s", provider, token, webhook, out.String())
				}
				server.Close()
			}
		}
	}
	for _, access := range []bool{false, true} {
		for _, active := range []bool{false, true} {
			for _, subscription := range []bool{false, true} {
				for _, offerURL := range []string{"absent", "", "https://example.test/pay"} {
					var sub any
					if subscription {
						sub = map[string]any{"active": active, "status": "arbitrary", "ends_at": "expired"}
					}
					var offer any
					if offerURL != "absent" {
						offer = map[string]any{"checkout_url": offerURL}
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						_ = json.NewEncoder(w).Encode(map[string]any{"has_access": access, "subscription": sub, "active_offer": offer})
					}))
					var out bytes.Buffer
					opened := 0
					a := &app{apiCli: client.New(server.URL, ""), printer: &output.Printer{Out: &out}, openURL: func(string) { opened++ }}
					c := billingStatusCmd(a)
					if err := formalMutationRun(c, nil); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(out.String(), "Paid access granted") != access {
						t.Fatalf("billing access derived from subscription instead of has_access: %q", out.String())
					}
					checkout := billingCheckoutCmd(a)
					err := formalMutationRun(checkout, nil)
					if offerURL == "" || offerURL == "absent" {
						if err == nil || opened != 0 {
							t.Fatal("missing checkout URL accepted")
						}
					} else if err != nil || opened != 1 {
						t.Fatalf("valid checkout rejected: %v opens=%d", err, opened)
					}
					server.Close()
				}
			}
		}
	}
}
