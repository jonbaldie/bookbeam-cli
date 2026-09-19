package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type BillingStatusResponse struct {
	HasAccess    bool                 `json:"has_access"`
	Subscription *BillingSubscription `json:"subscription"`
	ActiveOffer  *BillingActiveOffer  `json:"active_offer"`
}

type BillingSubscription struct {
	Active            bool   `json:"active"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	EndsAt            string `json:"ends_at"`
	CustomerPortalURL string `json:"customer_portal_url"`
}

type BillingActiveOffer struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceLabel     string `json:"price_label"`
	IsSubscription bool   `json:"is_subscription"`
	CheckoutURL    string `json:"checkout_url"`
}

func billingCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Check team account billing and subscription entitlement",
	}
	cmd.AddCommand(billingStatusCmd(a), billingCheckoutCmd(a))
	return cmd
}

func formatSubscription(s *BillingSubscription) (string, string) {
	if s == nil {
		return "none", "none"
	}
	name := s.Name
	if name == "" {
		name = "none"
	}
	status := s.Status
	if status == "" {
		status = "none"
	}
	return name, status
}

func formatActiveOffer(o *BillingActiveOffer) (string, string) {
	if o == nil {
		return "none", ""
	}
	if o.Name != "" && o.PriceLabel != "" {
		return fmt.Sprintf("%s (%s)", o.Name, o.PriceLabel), o.CheckoutURL
	}
	if o.Name != "" {
		return o.Name, o.CheckoutURL
	}
	if o.PriceLabel != "" {
		return o.PriceLabel, o.CheckoutURL
	}
	return "none", o.CheckoutURL
}

func billingStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current team subscription status and active plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Get("/api/v1/billing", nil)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			var b BillingStatusResponse
			if err := json.Unmarshal(raw, &b); err != nil {
				return err
			}

			accessStr := "Inactive (No access)"
			if b.HasAccess {
				accessStr = "Active (Paid access granted)"
			}

			subName, subStatus := formatSubscription(b.Subscription)
			offerStr, checkoutURL := formatActiveOffer(b.ActiveOffer)

			rows := [][]string{
				{"Access Status", accessStr},
				{"Subscription", subName},
				{"Subscription Status", subStatus},
				{"Active Offer", offerStr},
				{"Checkout Link", checkoutURL},
			}

			a.printer.Table([]string{"Field", "Value"}, rows)
			return nil
		},
	}
}

func billingCheckoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout",
		Short: "Open or display the checkout link for the active plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Get("/api/v1/billing", nil)
			if err != nil {
				return err
			}

			var b BillingStatusResponse
			if err := json.Unmarshal(raw, &b); err != nil || b.ActiveOffer == nil || b.ActiveOffer.CheckoutURL == "" {
				return fmt.Errorf("checkout unavailable: %s", string(raw))
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			a.printer.PrintInfo(fmt.Sprintf("Checkout URL: %s", b.ActiveOffer.CheckoutURL))
			a.openURL(b.ActiveOffer.CheckoutURL)
			return nil
		},
	}
}
