package cmd

import (
	"net/http"
	"strings"
	"testing"
)

const lifetimeDealResponse = `{
	"has_access": true,
	"subscription": {
		"active": false,
		"name": null,
		"status": null,
		"ends_at": null,
		"customer_portal_url": null
	},
	"active_offer": {
		"name": "Founder's Lifetime Deal",
		"description": "One-time payment. Lifetime access.",
		"price_label": "$99 one-time",
		"is_subscription": false,
		"checkout_url": "https://pay.example.com/checkout/123"
	}
}`

func TestIssue2BillingStatusRepro(t *testing.T) {
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, lifetimeDealResponse)
	}, false)

	err := arbRun(t, billingStatusCmd(h.a))
	if err != nil {
		t.Fatalf("billing status error: %v", err)
	}

	want := "Field                 Value\n" +
		"Access Status         Active (Paid access granted)\n" +
		"Subscription          none\n" +
		"Subscription Status   none\n" +
		"Active Offer          Founder's Lifetime Deal ($99 one-time)\n" +
		"Checkout Link         https://pay.example.com/checkout/123\n"
	if h.out.String() != want {
		t.Errorf("got\n%q\nwant\n%q", h.out.String(), want)
	}
}

func TestIssue2BillingCheckoutRepro(t *testing.T) {
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, lifetimeDealResponse)
	}, false)

	err := arbRun(t, billingCheckoutCmd(h.a))
	if err != nil {
		t.Fatalf("billing checkout error: %v", err)
	}

	if h.out.String() != "Checkout URL: https://pay.example.com/checkout/123\n" {
		t.Errorf("got %q", h.out.String())
	}
	if len(h.opened) != 1 || h.opened[0] != "https://pay.example.com/checkout/123" {
		t.Errorf("opened %v", h.opened)
	}
}

func TestIssue2BillingCheckoutNoURL(t *testing.T) {
	raw := `{"has_access":false,"subscription":null,"active_offer":{"name":"Free","checkout_url":""}}`
	h := arbNewHarness(t, func(w http.ResponseWriter, r *http.Request) {
		arbWriteJSON(w, 200, raw)
	}, false)

	err := arbRun(t, billingCheckoutCmd(h.a))
	if err == nil || !strings.Contains(err.Error(), "checkout unavailable: "+raw) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}
