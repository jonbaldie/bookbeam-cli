package cmd

import "testing"

func TestNewsletterWebhookWithoutURLChangesNothing(t *testing.T) {
	ts, seen := ntdServer(t, 200, `{"webhook_url":null}`)
	a, buf := ntdApp(ts.URL, false)
	cmd := newsletterWebhookCmd(a)
	err := cmd.RunE(cmd, nil)
	if err == nil || err.Error() != `--url is required (use --url "" to clear the webhook)` {
		t.Errorf("expected missing --url error, got %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("expected no API request, got %+v", *seen)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}
