package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type SignupLinkItem struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"book_project_id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	OptInText string `json:"opt_in_text"`
	CreatedAt string `json:"created_at"`
}

func linksCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "links",
		Aliases: []string{"link"},
		Short:   "Manage reader signup links and landing pages",
	}
	cmd.AddCommand(linksListCmd(a), linksCreateCmd(a), linksUpdateCmd(a), linksDeleteCmd(a))
	return cmd
}

func linksListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list <project-id>",
		Short: "List all signup links for a book project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/links", projectID), nil)
			if err != nil {
				return err
			}

			var response struct {
				Data []SignupLinkItem `json:"data"`
			}
			doc, err := decodeResponse(raw, &response)
			if err != nil {
				return err
			}

			var rows [][]string
			for _, l := range response.Data {
				publicURL := fmt.Sprintf("%s/download/%s", strings.TrimRight(a.cfg.Host, "/"), l.Slug)
				rows = append(rows, []string{
					strconv.Itoa(l.ID),
					l.Title,
					l.Slug,
					publicURL,
					l.CreatedAt,
				})
			}

			return a.printer.Display(output.View{
				Headers:     []string{"ID", "TITLE", "SLUG", "PUBLIC URL", "CREATED"},
				Rows:        rows,
				EmptyNotice: "No signup links found for this project.",
				Data:        doc,
			})
		},
	}
}

func fetchExistingLink(a *app, projectID, linkID string) (*SignupLinkItem, error) {
	raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/links", projectID), nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []SignupLinkItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}

	trimmedLinkID := strings.TrimSpace(linkID)
	for _, link := range response.Data {
		if strconv.Itoa(link.ID) == trimmedLinkID {
			return &link, nil
		}
	}

	return nil, fmt.Errorf("signup link #%s not found in project %s", linkID, projectID)
}

func linksCreateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <project-id>",
		Short: "Create a new reader signup link for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			title, _ := cmd.Flags().GetString("title")
			consent, _ := cmd.Flags().GetString("consent")

			payload := map[string]string{}
			if title != "" {
				payload["title"] = title
			}
			if consent != "" {
				payload["opt_in_text"] = consent
			}

			raw, err := a.apiCli.Post(fmt.Sprintf("/api/v1/projects/%s/links", projectID), payload)
			if err != nil {
				return err
			}

			var response struct {
				Data SignupLinkItem `json:"data"`
			}
			doc, err := decodeResponse(raw, &response)
			if err != nil {
				return err
			}
			created := response.Data
			publicURL := fmt.Sprintf("%s/download/%s", strings.TrimRight(a.cfg.Host, "/"), created.Slug)
			return a.printer.Success(doc, fmt.Sprintf("✓ Created signup link #%d: %s", created.ID, publicURL))
		},
	}
	cmd.Flags().String("title", "", "Signup link title")
	cmd.Flags().String("consent", "", "Newsletter consent text")
	return cmd
}

func linksUpdateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <project-id> <link-id>",
		Short: "Update an existing signup link",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			linkID := args[1]

			title, _ := cmd.Flags().GetString("title")
			consent, _ := cmd.Flags().GetString("consent")
			clearConsent, _ := cmd.Flags().GetBool("clear-consent")

			if consent != "" && clearConsent {
				return fmt.Errorf("cannot specify both --consent and --clear-consent")
			}

			payload, err := linkUpdatePayload(title, consent, clearConsent, func() (*SignupLinkItem, error) {
				return fetchExistingLink(a, projectID, linkID)
			})
			if err != nil {
				return err
			}

			raw, err := a.apiCli.Put(fmt.Sprintf("/api/v1/projects/%s/links/%s", projectID, linkID), payload)
			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Updated signup link #%s", linkID))
		},
	}
	cmd.Flags().String("title", "", "Updated link title")
	cmd.Flags().String("consent", "", "Updated newsletter consent text")
	cmd.Flags().Bool("clear-consent", false, "Clear newsletter consent text")
	return cmd
}

func linksDeleteCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <project-id> <link-id>",
		Short: "Delete a signup link",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			linkID := args[1]
			force, _ := cmd.Flags().GetBool("force")

			if !force && !a.printer.JSON {
				if !confirm(cmd, fmt.Sprintf("Are you sure you want to delete link #%s?", linkID)) {
					a.printer.Info("Cancelled.")
					return nil
				}
			}

			raw, err := a.apiCli.Delete(fmt.Sprintf("/api/v1/projects/%s/links/%s", projectID, linkID))
			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Deleted signup link #%s", linkID))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// linkUpdatePayload builds a full PUT body, filling fields the user left unset from the existing link.
func linkUpdatePayload(title, consent string, clearConsent bool, fetch func() (*SignupLinkItem, error)) (map[string]any, error) {
	if title == "" || (!clearConsent && consent == "") {
		existing, err := fetch()
		if err != nil {
			return nil, err
		}
		title = firstNonEmpty(title, existing.Title)
		if !clearConsent {
			consent = firstNonEmpty(consent, existing.OptInText)
		}
	}

	payload := map[string]any{"title": title}
	if clearConsent {
		// The API keeps fields a PUT omits, so clearing needs an explicit null.
		payload["opt_in_text"] = nil
	} else if consent != "" {
		payload["opt_in_text"] = consent
	}
	return payload, nil
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
