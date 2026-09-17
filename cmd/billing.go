package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type BillingStatusResponse struct {
	HasAccess   bool   `json:"has_access"`
	OfferType   string `json:"offer_type"`
	ActiveOffer string `json:"active_offer"`
	CheckoutURL string `json:"checkout_url"`
}

func billingCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Check team account billing and subscription entitlement",
	}
	cmd.AddCommand(billingStatusCmd(a), billingCheckoutCmd(a))
	return cmd
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

			rows := [][]string{
				{"Access Status", accessStr},
				{"Plan Type", strings.ToUpper(b.OfferType)},
				{"Active Offer", b.ActiveOffer},
				{"Checkout Link", b.CheckoutURL},
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
			if err := json.Unmarshal(raw, &b); err != nil || b.CheckoutURL == "" {
				return fmt.Errorf("checkout unavailable: %s", string(raw))
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			a.printer.PrintInfo(fmt.Sprintf("Checkout URL: %s", b.CheckoutURL))
			a.openURL(b.CheckoutURL)
			return nil
		},
	}
}
