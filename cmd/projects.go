package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type ProjectItem struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	CoverImageURL    string   `json:"cover_image_url"`
	FilesCount       int      `json:"files_count"`
	SignupLinksCount int      `json:"signup_links_count"`
	CreatedAt        string   `json:"created_at"`
	NewsletterListID string   `json:"newsletter_list_id"`
	NewsletterTags   []string `json:"newsletter_tags"`
}

type ProjectListResponse struct {
	Data        []ProjectItem `json:"data"`
	CurrentPage int           `json:"current_page"`
	LastPage    int           `json:"last_page"`
	Total       int           `json:"total"`
}

func validateProjectUpdateFlags(description, cover string, clearDescription, removeCover bool) error {
	if cover != "" && removeCover {
		return fmt.Errorf("cannot specify both --cover and --remove-cover")
	}
	if description != "" && clearDescription {
		return fmt.Errorf("cannot specify both --description and --clear-description")
	}
	return nil
}

func buildProjectUpdatePayload(title, description string, clearDescription, removeCover bool) map[string]any {
	payload := make(map[string]any)
	if title != "" {
		payload["title"] = title
	}
	if clearDescription {
		// The API keeps fields a PUT omits, so clearing needs an explicit null.
		payload["description"] = nil
	} else if description != "" {
		payload["description"] = description
	}
	if removeCover {
		payload["remove_cover_image"] = true
	}
	return payload
}

func buildProjectUpdateMultipartFields(title, description string, clearDescription, removeCover bool) map[string]string {
	fields := make(map[string]string)
	if title != "" {
		fields["title"] = title
	}
	if clearDescription {
		// Multipart cannot carry a null; the API stores an empty field as null.
		fields["description"] = ""
	} else if description != "" {
		fields["description"] = description
	}
	if removeCover {
		fields["remove_cover_image"] = "true"
	}
	fields["_method"] = "PUT"
	return fields
}

func projectsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"project"},
		Short:   "Manage book projects in your catalog",
	}
	cmd.AddCommand(projectsListCmd(a), projectsGetCmd(a), projectsCreateCmd(a), projectsUpdateCmd(a), projectsDeleteCmd(a), projectsNewsletterCmd(a))
	return cmd
}

func projectsListCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List book projects for your active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			page, _ := cmd.Flags().GetInt("page")
			query := url.Values{}
			if page > 0 {
				query.Set("page", strconv.Itoa(page))
			}

			raw, err := a.apiCli.Get("/api/v1/projects", query)
			if err != nil {
				return err
			}

			var listResp ProjectListResponse
			doc, err := decodeWithDocument(raw, &listResp)
			if err != nil {
				return err
			}

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

			return a.printer.Display(output.View{
				Headers:     []string{"ID", "TITLE", "FILES", "LINKS", "CREATED"},
				Rows:        rows,
				Footer:      fmt.Sprintf("\nPage %d of %d (Total: %d)", listResp.CurrentPage, listResp.LastPage, listResp.Total),
				EmptyNotice: "No book projects found. Create one with 'bookbeam projects create'.",
				Data:        doc,
			})
		},
	}
	cmd.Flags().Int("page", 1, "Page number")
	return cmd
}

func projectsGetCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "get <project-id>",
		Short: "Get details for a single book project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}
			raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%d", projectID), nil)
			if err != nil {
				return err
			}

			var response struct {
				Data ProjectItem `json:"data"`
			}
			doc, err := decodeWithDocument(raw, &response)
			if err != nil {
				return err
			}
			p := response.Data

			return a.printer.Display(output.View{
				Headers: []string{"Field", "Value"},
				Rows: [][]string{
					{"ID", strconv.Itoa(p.ID)},
					{"Title", p.Title},
					{"Description", p.Description},
					{"Cover URL", p.CoverImageURL},
					{"Files Count", strconv.Itoa(p.FilesCount)},
					{"Links Count", strconv.Itoa(p.SignupLinksCount)},
					{"Created At", p.CreatedAt},
				},
				Data: doc,
			})
		},
	}
}

func projectsCreateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
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
				raw, err = a.apiCli.PostMultipart("/api/v1/projects", fields, "cover_image", cover)
			} else {
				payload := map[string]string{
					"title": title,
				}
				if description != "" {
					payload["description"] = description
				}
				raw, err = a.apiCli.Post("/api/v1/projects", payload)
			}

			if err != nil {
				return err
			}

			var response struct {
				Data ProjectItem `json:"data"`
			}
			doc, err := decodeWithDocument(raw, &response)
			if err != nil {
				return err
			}
			created := response.Data
			return a.printer.Success(doc, fmt.Sprintf("✓ Created book project #%d: %s", created.ID, created.Title))
		},
	}
	cmd.Flags().String("title", "", "Book project title (required)")
	cmd.Flags().String("description", "", "Book project description")
	cmd.Flags().String("cover", "", "Path to cover image file")
	return cmd
}

func projectsUpdateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <project-id>",
		Short: "Update an existing book project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}
			title, _ := cmd.Flags().GetString("title")
			description, _ := cmd.Flags().GetString("description")
			cover, _ := cmd.Flags().GetString("cover")
			removeCover, _ := cmd.Flags().GetBool("remove-cover")
			clearDescription, _ := cmd.Flags().GetBool("clear-description")

			if err := validateProjectUpdateFlags(description, cover, clearDescription, removeCover); err != nil {
				return err
			}

			var raw []byte

			if cover != "" {
				fields := buildProjectUpdateMultipartFields(title, description, clearDescription, removeCover)
				raw, err = a.apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%d", projectID), fields, "cover_image", cover)
			} else {
				payload := buildProjectUpdatePayload(title, description, clearDescription, removeCover)
				raw, err = a.apiCli.Put(fmt.Sprintf("/api/v1/projects/%d", projectID), payload)
			}

			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Updated book project #%d", projectID))
		},
	}
	cmd.Flags().String("title", "", "Updated book project title")
	cmd.Flags().String("description", "", "Updated book project description")
	cmd.Flags().Bool("clear-description", false, "Clear the book project description")
	cmd.Flags().String("cover", "", "Path to new cover image file")
	cmd.Flags().Bool("remove-cover", false, "Remove current cover image")
	return cmd
}

func projectsDeleteCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <project-id>",
		Short: "Delete a book project and its attached files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")

			if !a.printer.Proceed(force, fmt.Sprintf("Are you sure you want to delete project #%d?", projectID)) {
				return nil
			}

			raw, err := a.apiCli.Delete(fmt.Sprintf("/api/v1/projects/%d", projectID))
			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Deleted book project #%d", projectID))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func fetchExistingProject(a *app, projectID int) (*ProjectItem, error) {
	raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%d", projectID), nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data ProjectItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func projectNewsletterPayload(listID, tags string, clearTags bool, fetch func() (*ProjectItem, error)) (map[string]any, error) {
	if listID == "" || (!clearTags && tags == "") {
		existing, err := fetch()
		if err != nil {
			return nil, err
		}
		listID = firstNonEmpty(listID, existing.NewsletterListID)
		if !clearTags {
			tags = firstNonEmpty(tags, strings.Join(existing.NewsletterTags, ","))
		}
	}

	payload := map[string]any{"newsletter_list_id": listID}
	if clearTags {
		payload["newsletter_tags"] = nil
	} else if tags != "" {
		payload["newsletter_tags"] = tags
	}
	return payload, nil
}

func projectsNewsletterCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "newsletter <project-id>",
		Short: "Set project newsletter list and tags routing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}
			listID, _ := cmd.Flags().GetString("list-id")
			tags, _ := cmd.Flags().GetString("tags")
			clearTags, _ := cmd.Flags().GetBool("clear-tags")

			if tags != "" && clearTags {
				return fmt.Errorf("cannot specify both --tags and --clear-tags")
			}
			if listID == "" && tags == "" && !clearTags {
				return fmt.Errorf("specify --list-id, --tags, or --clear-tags")
			}

			payload, err := projectNewsletterPayload(listID, tags, clearTags, func() (*ProjectItem, error) {
				return fetchExistingProject(a, projectID)
			})
			if err != nil {
				return err
			}

			raw, err := a.apiCli.Put(fmt.Sprintf("/api/v1/projects/%d/newsletter", projectID), payload)
			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Updated newsletter routing for project #%d", projectID))
		},
	}
	cmd.Flags().String("list-id", "", "Newsletter list ID")
	cmd.Flags().String("tags", "", "Comma-separated newsletter tags")
	cmd.Flags().Bool("clear-tags", false, "Clear newsletter tags")
	return cmd
}
