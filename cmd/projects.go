package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagProjectPage        int
	flagProjectTitle       string
	flagProjectDescription string
	flagProjectCover       string
	flagProjectRemoveCover bool
	flagProjectForce       bool
	flagProjectListID      string
	flagProjectTags        string
)

type ProjectItem struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CoverImageURL  string `json:"cover_image_url"`
	FilesCount     int    `json:"files_count"`
	SignupLinksCount int  `json:"signup_links_count"`
	CreatedAt      string `json:"created_at"`
}

type ProjectListResponse struct {
	Data        []ProjectItem `json:"data"`
	CurrentPage int           `json:"current_page"`
	LastPage    int           `json:"last_page"`
	Total       int           `json:"total"`
}

var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project"},
	Short:   "Manage book projects in your catalog",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List book projects for your active team",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := url.Values{}
		if flagProjectPage > 0 {
			query.Set("page", strconv.Itoa(flagProjectPage))
		}

		raw, err := apiCli.Get("/api/v1/projects", query)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var listResp ProjectListResponse
		if err := json.Unmarshal(raw, &listResp); err != nil {
			return err
		}

		if len(listResp.Data) == 0 {
			printer.PrintInfo("No book projects found. Create one with 'bookbeam projects create'.")
			return nil
		}

		headers := []string{"ID", "TITLE", "FILES", "LINKS", "CREATED"}
		var rows [][]string
		for _, p := range listResp.Data {
			rows = append(rows, []string{
				strconv.Itoa(p.ID),
				p.Title,
				strconv.Itoa(p.FilesCount),
				strconv.Itoa(p.SignupLinksCount),
				p.CreatedAt,
			})
		}

		printer.Table(headers, rows)
		printer.PrintInfo(fmt.Sprintf("\nPage %d of %d (Total: %d)", listResp.CurrentPage, listResp.LastPage, listResp.Total))
		return nil
	},
}

var projectsGetCmd = &cobra.Command{
	Use:   "get <project-id>",
	Short: "Get details for a single book project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		raw, err := apiCli.Get(fmt.Sprintf("/api/v1/projects/%s", projectID), nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var p ProjectItem
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}

		rows := [][]string{
			{"ID", strconv.Itoa(p.ID)},
			{"Title", p.Title},
			{"Description", p.Description},
			{"Cover URL", p.CoverImageURL},
			{"Files Count", strconv.Itoa(p.FilesCount)},
			{"Links Count", strconv.Itoa(p.SignupLinksCount)},
			{"Created At", p.CreatedAt},
		}

		printer.Table([]string{"Field", "Value"}, rows)
		return nil
	},
}

var projectsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new book project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagProjectTitle == "" {
			return fmt.Errorf("--title is required")
		}

		var raw []byte
		var err error

		if flagProjectCover != "" {
			fields := map[string]string{
				"title": flagProjectTitle,
			}
			if flagProjectDescription != "" {
				fields["description"] = flagProjectDescription
			}
			raw, err = apiCli.PostMultipart("/api/v1/projects", fields, "cover_image", flagProjectCover)
		} else {
			payload := map[string]string{
				"title": flagProjectTitle,
			}
			if flagProjectDescription != "" {
				payload["description"] = flagProjectDescription
			}
			raw, err = apiCli.Post("/api/v1/projects", payload)
		}

		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var created ProjectItem
		_ = json.Unmarshal(raw, &created)
		printer.PrintInfo(fmt.Sprintf("✓ Created book project #%d: %s", created.ID, created.Title))
		return nil
	},
}

var projectsUpdateCmd = &cobra.Command{
	Use:   "update <project-id>",
	Short: "Update an existing book project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		payload := make(map[string]any)

		if flagProjectTitle != "" {
			payload["title"] = flagProjectTitle
		}
		if flagProjectDescription != "" {
			payload["description"] = flagProjectDescription
		}
		if flagProjectRemoveCover {
			payload["remove_cover"] = true
		}

		var raw []byte
		var err error

		if flagProjectCover != "" {
			fields := map[string]string{}
			if flagProjectTitle != "" {
				fields["title"] = flagProjectTitle
			}
			if flagProjectDescription != "" {
				fields["description"] = flagProjectDescription
			}
			raw, err = apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%s", projectID), fields, "cover_image", flagProjectCover)
		} else {
			raw, err = apiCli.Put(fmt.Sprintf("/api/v1/projects/%s", projectID), payload)
		}

		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Updated book project #%s", projectID))
		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete <project-id>",
	Short: "Delete a book project and its attached files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]

		if !flagProjectForce && !printer.JSON {
			fmt.Printf("Are you sure you want to delete project #%s? (y/N): ", projectID)
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans != "y" && ans != "yes" {
					printer.PrintInfo("Cancelled.")
					return nil
				}
			}
		}

		raw, err := apiCli.Delete(fmt.Sprintf("/api/v1/projects/%s", projectID))
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Deleted book project #%s", projectID))
		return nil
	},
}

var projectsNewsletterCmd = &cobra.Command{
	Use:   "newsletter <project-id>",
	Short: "Set project newsletter list and tags routing",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		if flagProjectListID == "" {
			return fmt.Errorf("--list-id is required")
		}

		payload := map[string]any{
			"newsletter_list_id": flagProjectListID,
		}
		if flagProjectTags != "" {
			payload["newsletter_tags"] = flagProjectTags
		}

		raw, err := apiCli.Put(fmt.Sprintf("/api/v1/projects/%s/newsletter", projectID), payload)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Updated newsletter routing for project #%s", projectID))
		return nil
	},
}

func init() {
	projectsListCmd.Flags().IntVar(&flagProjectPage, "page", 1, "Page number")

	projectsCreateCmd.Flags().StringVar(&flagProjectTitle, "title", "", "Book project title (required)")
	projectsCreateCmd.Flags().StringVar(&flagProjectDescription, "description", "", "Book project description")
	projectsCreateCmd.Flags().StringVar(&flagProjectCover, "cover", "", "Path to cover image file")

	projectsUpdateCmd.Flags().StringVar(&flagProjectTitle, "title", "", "Updated book project title")
	projectsUpdateCmd.Flags().StringVar(&flagProjectDescription, "description", "", "Updated book project description")
	projectsUpdateCmd.Flags().StringVar(&flagProjectCover, "cover", "", "Path to new cover image file")
	projectsUpdateCmd.Flags().BoolVar(&flagProjectRemoveCover, "remove-cover", false, "Remove current cover image")

	projectsDeleteCmd.Flags().BoolVarP(&flagProjectForce, "force", "f", false, "Skip confirmation prompt")

	projectsNewsletterCmd.Flags().StringVar(&flagProjectListID, "list-id", "", "Newsletter list ID (required)")
	projectsNewsletterCmd.Flags().StringVar(&flagProjectTags, "tags", "", "Comma-separated newsletter tags")

	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsUpdateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsNewsletterCmd)

	rootCmd.AddCommand(projectsCmd)
}
