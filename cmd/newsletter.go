package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagNewsletterProvider string
	flagNewsletterAPIKey   string
	flagNewsletterEndpoint string
	flagNewsletterWebhook  string
)

type NewsletterSettingsResponse struct {
	Provider     string `json:"provider"`
	WebhookURL   string `json:"webhook_url"`
	IsConfigured bool   `json:"is_configured"`
}

type NewsletterListOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NewsletterListsResponse struct {
	Lists []NewsletterListOption `json:"lists"`
	Tags  []NewsletterListOption `json:"tags"`
}

var newsletterCmd = &cobra.Command{
	Use:   "newsletter",
	Short: "Manage mailing list integrations and webhook notifications",
}

var newsletterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current team newsletter provider settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Get("/api/v1/settings/newsletter", nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var settings NewsletterSettingsResponse
		if err := json.Unmarshal(raw, &settings); err != nil {
			return err
		}

		provider := settings.Provider
		if provider == "" {
			provider = "None (disconnected)"
		}

		webhook := settings.WebhookURL
		if webhook == "" {
			webhook = "None"
		}

		configured := "No"
		if settings.IsConfigured {
			configured = "Yes"
		}

		rows := [][]string{
			{"Active Provider", strings.ToUpper(provider)},
			{"Configured", configured},
			{"Webhook Endpoint", webhook},
		}

		printer.Table([]string{"Setting", "Value"}, rows)
		return nil
	},
}

var newsletterConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Connect a newsletter service (mailerlite, kit, mailcoach)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagNewsletterProvider == "" {
			return fmt.Errorf("--provider is required (kit, mailcoach, or mailerlite)")
		}
		if flagNewsletterAPIKey == "" {
			return fmt.Errorf("--api-key is required")
		}

		payload := map[string]string{
			"provider": strings.ToLower(flagNewsletterProvider),
			"api_key":  flagNewsletterAPIKey,
		}
		if flagNewsletterEndpoint != "" {
			payload["api_endpoint"] = flagNewsletterEndpoint
		}

		raw, err := apiCli.Put("/api/v1/settings/newsletter/provider", payload)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Connected provider: %s", strings.ToUpper(flagNewsletterProvider)))
		return nil
	},
}

var newsletterDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect active newsletter provider credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Delete("/api/v1/settings/newsletter/provider")
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo("✓ Disconnected newsletter provider.")
		return nil
	},
}

var newsletterWebhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Set or clear subscriber notification webhook URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]string{
			"webhook_url": flagNewsletterWebhook,
		}

		raw, err := apiCli.Put("/api/v1/settings/newsletter/webhook", payload)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		if flagNewsletterWebhook == "" {
			printer.PrintInfo("✓ Cleared newsletter webhook URL.")
		} else {
			printer.PrintInfo(fmt.Sprintf("✓ Configured webhook URL: %s", flagNewsletterWebhook))
		}
		return nil
	},
}

var newsletterListsCmd = &cobra.Command{
	Use:   "lists",
	Short: "Fetch available lists and tags from configured provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Post("/api/v1/settings/newsletter/lists", map[string]string{})
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var listsResp NewsletterListsResponse
		if err := json.Unmarshal(raw, &listsResp); err != nil {
			return err
		}

		if len(listsResp.Lists) == 0 && len(listsResp.Tags) == 0 {
			printer.PrintInfo("No mailing lists or tags returned by provider.")
			return nil
		}

		if len(listsResp.Lists) > 0 {
			printer.PrintInfo("Mailing Lists:")
			var rows [][]string
			for _, l := range listsResp.Lists {
				rows = append(rows, []string{l.ID, l.Name})
			}
			printer.Table([]string{"LIST ID", "NAME"}, rows)
		}

		if len(listsResp.Tags) > 0 {
			printer.PrintInfo("\nTags:")
			var rows [][]string
			for _, t := range listsResp.Tags {
				rows = append(rows, []string{t.ID, t.Name})
			}
			printer.Table([]string{"TAG ID", "NAME"}, rows)
		}

		return nil
	},
}

func init() {
	newsletterConfigureCmd.Flags().StringVar(&flagNewsletterProvider, "provider", "", "Provider type: kit, mailcoach, or mailerlite (required)")
	newsletterConfigureCmd.Flags().StringVar(&flagNewsletterAPIKey, "api-key", "", "Provider API secret key (required)")
	newsletterConfigureCmd.Flags().StringVar(&flagNewsletterEndpoint, "endpoint", "", "API endpoint URL (required for Mailcoach)")

	newsletterWebhookCmd.Flags().StringVar(&flagNewsletterWebhook, "url", "", "Target webhook URL (empty string clears webhook)")

	newsletterCmd.AddCommand(newsletterStatusCmd)
	newsletterCmd.AddCommand(newsletterConfigureCmd)
	newsletterCmd.AddCommand(newsletterDisconnectCmd)
	newsletterCmd.AddCommand(newsletterWebhookCmd)
	newsletterCmd.AddCommand(newsletterListsCmd)

	rootCmd.AddCommand(newsletterCmd)
}
