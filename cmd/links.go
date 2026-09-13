package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagLinkTitle   string
	flagLinkOptIn   string
	flagLinkForce   bool
)

type SignupLinkItem struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"book_project_id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	OptInText string `json:"opt_in_text"`
	CreatedAt string `json:"created_at"`
}

var linksCmd = &cobra.Command{
	Use:     "links",
	Aliases: []string{"link"},
	Short:   "Manage reader signup links and landing pages",
}

var linksListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "List all signup links for a book project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		raw, err := apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/links", projectID), nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var links []SignupLinkItem
		if err := json.Unmarshal(raw, &links); err != nil {
			return err
		}

		if len(links) == 0 {
			printer.PrintInfo("No signup links found for this project.")
			return nil
		}

		headers := []string{"ID", "TITLE", "SLUG", "PUBLIC URL", "CREATED"}
		var rows [][]string
		for _, l := range links {
			publicURL := fmt.Sprintf("%s/download/%s", strings.TrimRight(cfg.Host, "/"), l.Slug)
			rows = append(rows, []string{
				strconv.Itoa(l.ID),
				l.Title,
				l.Slug,
				publicURL,
				l.CreatedAt,
			})
		}

		printer.Table(headers, rows)
		return nil
	},
}

var linksCreateCmd = &cobra.Command{
	Use:   "create <project-id>",
	Short: "Create a new reader signup link for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		payload := map[string]string{}
		if flagLinkTitle != "" {
			payload["title"] = flagLinkTitle
		}
		if flagLinkOptIn != "" {
			payload["opt_in_text"] = flagLinkOptIn
		}

		raw, err := apiCli.Post(fmt.Sprintf("/api/v1/projects/%s/links", projectID), payload)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var created SignupLinkItem
		_ = json.Unmarshal(raw, &created)
		publicURL := fmt.Sprintf("%s/download/%s", strings.TrimRight(cfg.Host, "/"), created.Slug)
		printer.PrintInfo(fmt.Sprintf("✓ Created signup link #%d: %s", created.ID, publicURL))
		return nil
	},
}

var linksUpdateCmd = &cobra.Command{
	Use:   "update <project-id> <link-id>",
	Short: "Update an existing signup link",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		linkID := args[1]

		payload := map[string]string{}
		if flagLinkTitle != "" {
			payload["title"] = flagLinkTitle
		}
		if flagLinkOptIn != "" {
			payload["opt_in_text"] = flagLinkOptIn
		}

		raw, err := apiCli.Put(fmt.Sprintf("/api/v1/projects/%s/links/%s", projectID, linkID), payload)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Updated signup link #%s", linkID))
		return nil
	},
}

var linksDeleteCmd = &cobra.Command{
	Use:   "delete <project-id> <link-id>",
	Short: "Delete a signup link",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		linkID := args[1]

		if !flagLinkForce && !printer.JSON {
			fmt.Printf("Are you sure you want to delete link #%s? (y/N): ", linkID)
			var ans string
			fmt.Scanln(&ans)
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "y" && ans != "yes" {
				printer.PrintInfo("Cancelled.")
				return nil
			}
		}

		raw, err := apiCli.Delete(fmt.Sprintf("/api/v1/projects/%s/links/%s", projectID, linkID))
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Deleted signup link #%s", linkID))
		return nil
	},
}

func init() {
	linksCreateCmd.Flags().StringVar(&flagLinkTitle, "title", "", "Signup link title")
	linksCreateCmd.Flags().StringVar(&flagLinkOptIn, "consent", "", "Newsletter consent text")

	linksUpdateCmd.Flags().StringVar(&flagLinkTitle, "title", "", "Updated link title")
	linksUpdateCmd.Flags().StringVar(&flagLinkOptIn, "consent", "", "Updated newsletter consent text")

	linksDeleteCmd.Flags().BoolVarP(&flagLinkForce, "force", "f", false, "Skip confirmation prompt")

	linksCmd.AddCommand(linksListCmd)
	linksCmd.AddCommand(linksCreateCmd)
	linksCmd.AddCommand(linksUpdateCmd)
	linksCmd.AddCommand(linksDeleteCmd)

	rootCmd.AddCommand(linksCmd)
}
