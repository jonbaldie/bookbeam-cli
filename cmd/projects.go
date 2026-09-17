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

type ProjectItem struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	CoverImageURL    string `json:"cover_image_url"`
	FilesCount       int    `json:"files_count"`
	SignupLinksCount int    `json:"signup_links_count"`
	CreatedAt        string `json:"created_at"`
}

type ProjectListResponse struct {
	Data        []ProjectItem `json:"data"`
	CurrentPage int           `json:"current_page"`
	LastPage    int           `json:"last_page"`
	Total       int           `json:"total"`
}

func buildProjectUpdateMultipartFields(title, description string, removeCover bool) map[string]string {
	fields := make(map[string]string)
	if title != "" {
		fields["title"] = title
	}
	if description != "" {
		fields["description"] = description
	}
	if removeCover {
		fields["remove_cover"] = "true"
	}
	fields["_method"] = "PUT"
	return fields
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
		page, _ := cmd.Flags().GetInt("page")
		query := url.Values{}
		if page > 0 {
			query.Set("page", strconv.Itoa(page))
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

		var response struct {
			Data ProjectItem `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return err
		}
		p := response.Data

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
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		cover, _ := cmd.Flags().GetString("cover")

		if title == "" {
			return fmt.Errorf("--title is required")
		}

		var raw []byte
		var err error

		if cover != "" {
			fields := map[string]string{
				"title": title,
			}
			if description != "" {
				fields["description"] = description
			}
			raw, err = apiCli.PostMultipart("/api/v1/projects", fields, "cover_image", cover)
		} else {
			payload := map[string]string{
				"title": title,
			}
			if description != "" {
				payload["description"] = description
			}
			raw, err = apiCli.Post("/api/v1/projects", payload)
		}

		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var response struct {
			Data ProjectItem `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return err
		}
		created := response.Data
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
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		cover, _ := cmd.Flags().GetString("cover")
		removeCover, _ := cmd.Flags().GetBool("remove-cover")

		if cover != "" && removeCover {
			return fmt.Errorf("cannot specify both --cover and --remove-cover")
		}

		payload := make(map[string]any)

		if title != "" {
			payload["title"] = title
		}
		if description != "" {
			payload["description"] = description
		}
		if removeCover {
			payload["remove_cover"] = true
		}

		var raw []byte
		var err error

		if cover != "" {
			fields := buildProjectUpdateMultipartFields(title, description, removeCover)
			raw, err = apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%s", projectID), fields, "cover_image", cover)
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
		force, _ := cmd.Flags().GetBool("force")

		if !force && !printer.JSON {
			fmt.Printf("Are you sure you want to delete project #%s? (y/N): ", projectID)
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				printer.PrintInfo("Cancelled.")
				return nil
			}
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans != "y" && ans != "yes" {
				printer.PrintInfo("Cancelled.")
				return nil
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
		listID, _ := cmd.Flags().GetString("list-id")
		tags, _ := cmd.Flags().GetString("tags")

		if listID == "" {
			return fmt.Errorf("--list-id is required")
		}

		payload := map[string]any{
			"newsletter_list_id": listID,
		}
		if tags != "" {
			payload["newsletter_tags"] = tags
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
	projectsListCmd.Flags().Int("page", 1, "Page number")

	projectsCreateCmd.Flags().String("title", "", "Book project title (required)")
	projectsCreateCmd.Flags().String("description", "", "Book project description")
	projectsCreateCmd.Flags().String("cover", "", "Path to cover image file")

	projectsUpdateCmd.Flags().String("title", "", "Updated book project title")
	projectsUpdateCmd.Flags().String("description", "", "Updated book project description")
	projectsUpdateCmd.Flags().String("cover", "", "Path to new cover image file")
	projectsUpdateCmd.Flags().Bool("remove-cover", false, "Remove current cover image")

	projectsDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	projectsNewsletterCmd.Flags().String("list-id", "", "Newsletter list ID (required)")
	projectsNewsletterCmd.Flags().String("tags", "", "Comma-separated newsletter tags")

	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsUpdateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsNewsletterCmd)

	rootCmd.AddCommand(projectsCmd)
}
