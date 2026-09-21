package cmd

import (
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

			var settings NewsletterSettingsResponse
			doc, err := decodeResponse(raw, &settings)
			if err != nil {
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

			return a.printer.Display(output.View{
				Headers: []string{"Setting", "Value"},
				Rows: [][]string{
					{"Active Provider", strings.ToUpper(provider)},
					{"Configured", configured},
					{"Webhook Endpoint", webhook},
				},
				Data: doc,
			})
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

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Connected provider: %s", strings.ToUpper(provider)))
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

			return a.printer.Success(responseDocument(raw), "✓ Disconnected newsletter provider.")
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

			msg := "✓ Cleared newsletter webhook URL."
			if webhookURL != "" {
				msg = fmt.Sprintf("✓ Configured webhook URL: %s", webhookURL)
			}
			return a.printer.Success(responseDocument(raw), msg)
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

			var listsResp NewsletterListsResponse
			doc, err := decodeResponse(raw, &listsResp)
			if err != nil {
				return err
			}

			rows := make([][]string, 0, len(listsResp.Data))
			for _, option := range listsResp.Data {
				rows = append(rows, []string{option.ID, option.Name})
			}

			return a.printer.Display(output.View{
				Title:       "Mailing Lists:",
				Headers:     []string{"LIST ID", "NAME"},
				Rows:        rows,
				EmptyNotice: "No mailing lists returned by provider.",
				Data:        doc,
			})
		},
	}
}
