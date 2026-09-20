package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type NewsletterProviderConfig struct {
	APIToken string `json:"api_token"`
}

type NewsletterSettingsResponse struct {
	Provider   string                   `json:"provider"`
	WebhookURL string                   `json:"webhook_url"`
	Config     NewsletterProviderConfig `json:"config"`
}

type NewsletterListOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NewsletterListsResponse struct {
	Data []NewsletterListOption `json:"data"`
}

func newsletterCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "newsletter",
		Short: "Manage mailing list integrations and webhook notifications",
	}
	cmd.AddCommand(newsletterStatusCmd(a), newsletterConfigureCmd(a), newsletterDisconnectCmd(a), newsletterWebhookCmd(a), newsletterListsCmd(a))
	return cmd
}

func newsletterStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current team newsletter provider settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Get("/api/v1/settings/newsletter", nil)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
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
			if settings.Provider != "" && settings.Config.APIToken != "" {
				configured = "Yes"
			}

			rows := [][]string{
				{"Active Provider", strings.ToUpper(provider)},
				{"Configured", configured},
				{"Webhook Endpoint", webhook},
			}

			a.printer.Table([]string{"Setting", "Value"}, rows)
			return nil
		},
	}
}

func newsletterConfigureCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Connect a newsletter service (mailerlite, kit, mailcoach)",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _ := cmd.Flags().GetString("provider")
			apiKey, _ := cmd.Flags().GetString("api-key")
			endpoint, _ := cmd.Flags().GetString("endpoint")
			if provider == "" {
				return fmt.Errorf("--provider is required (kit, mailcoach, or mailerlite)")
			}
			if apiKey == "" {
				return fmt.Errorf("--api-key is required")
			}

			payload := map[string]string{
				"provider":  strings.ToLower(provider),
				"api_token": apiKey,
			}
			if endpoint != "" {
				payload["api_url"] = endpoint
			}

			raw, err := a.apiCli.Put("/api/v1/settings/newsletter/provider", payload)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			a.printer.PrintInfo(fmt.Sprintf("✓ Connected provider: %s", strings.ToUpper(provider)))
			return nil
		},
	}
	cmd.Flags().String("provider", "", "Provider type: kit, mailcoach, or mailerlite (required)")
	cmd.Flags().String("api-key", "", "Provider API secret key (required)")
	cmd.Flags().String("endpoint", "", "API endpoint URL (required for Mailcoach)")
	return cmd
}

func newsletterDisconnectCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect active newsletter provider credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Delete("/api/v1/settings/newsletter/provider")
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			a.printer.PrintInfo("✓ Disconnected newsletter provider.")
			return nil
		},
	}
}

func newsletterWebhookCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Set or clear subscriber notification webhook URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			webhookURL, _ := cmd.Flags().GetString("url")
			payload := map[string]string{
				"webhook_url": webhookURL,
			}

			raw, err := a.apiCli.Put("/api/v1/settings/newsletter/webhook", payload)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			if webhookURL == "" {
				a.printer.PrintInfo("✓ Cleared newsletter webhook URL.")
			} else {
				a.printer.PrintInfo(fmt.Sprintf("✓ Configured webhook URL: %s", webhookURL))
			}
			return nil
		},
	}
	cmd.Flags().String("url", "", "Target webhook URL (empty string clears webhook)")
	return cmd
}

func newsletterListsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "lists",
		Short: "Fetch available lists from configured provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Post("/api/v1/settings/newsletter/lists", map[string]string{})
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			var listsResp NewsletterListsResponse
			if err := json.Unmarshal(raw, &listsResp); err != nil {
				return err
			}

			if len(listsResp.Data) == 0 {
				a.printer.PrintInfo("No mailing lists returned by provider.")
				return nil
			}

			printOptions(a.printer, "Mailing Lists:", "LIST ID", listsResp.Data)

			return nil
		},
	}
}

func printOptions(printer *output.Printer, title, idHeader string, options []NewsletterListOption) {
	if len(options) == 0 {
		return
	}
	printer.PrintInfo(title)
	rows := make([][]string, 0, len(options))
	for _, option := range options {
		rows = append(rows, []string{option.ID, option.Name})
	}
	printer.Table([]string{idHeader, "NAME"}, rows)
}
