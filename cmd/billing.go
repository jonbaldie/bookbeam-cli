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

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Check team account billing and subscription entitlement",
}

var billingStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current team subscription status and active plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Get("/api/v1/billing", nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
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

		printer.Table([]string{"Field", "Value"}, rows)
		return nil
	},
}

var billingCheckoutCmd = &cobra.Command{
	Use:   "checkout",
	Short: "Open or display the checkout link for the active plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Get("/api/v1/billing", nil)
		if err != nil {
			return err
		}

		var b BillingStatusResponse
		if err := json.Unmarshal(raw, &b); err != nil || b.CheckoutURL == "" {
			return fmt.Errorf("checkout unavailable: %s", string(raw))
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("Checkout URL: %s", b.CheckoutURL))
		openBrowser(b.CheckoutURL)
		return nil
	},
}

func init() {
	billingCmd.AddCommand(billingStatusCmd)
	billingCmd.AddCommand(billingCheckoutCmd)

	rootCmd.AddCommand(billingCmd)
}
