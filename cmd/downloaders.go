package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type DownloaderItem struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	LinkName  string `json:"signup_link_title"`
	CreatedAt string `json:"signed_up_at"`
}

type DownloaderListResponse struct {
	Data        []DownloaderItem `json:"data"`
	CurrentPage int              `json:"current_page"`
	LastPage    int              `json:"last_page"`
	Total       int              `json:"total"`
}

func downloadersCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "downloaders",
		Aliases: []string{"downloader"},
		Short:   "View reader downloaders and export subscriber lists",
	}
	cmd.AddCommand(downloadersListCmd(a), downloadersExportCmd(a))
	return cmd
}

func downloadersListCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project-id>",
		Short: "List downloader subscribers for a book project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			query := url.Values{}
			page, _ := cmd.Flags().GetInt("page")
			if page > 0 {
				query.Set("page", strconv.Itoa(page))
			}

			raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/downloaders", projectID), query)
			if err != nil {
				return err
			}

			var resp DownloaderListResponse
			doc, err := decodeWithDocument(raw, &resp)
			if err != nil {
				return err
			}

			var rows [][]string
			for _, d := range resp.Data {
				rows = append(rows, []string{
					d.Email,
					d.LinkName,
					d.CreatedAt,
				})
			}

			return a.printer.Display(output.View{
				Headers:     []string{"EMAIL", "SIGNUP LINK", "SIGNUP DATE"},
				Rows:        rows,
				Footer:      fmt.Sprintf("\nPage %d of %d (Total: %d)", resp.CurrentPage, resp.LastPage, resp.Total),
				EmptyNotice: "No downloaders found for this project.",
				Data:        doc,
			})
		},
	}
	cmd.Flags().Int("page", 1, "Page number")
	return cmd
}

func downloadersExportCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <project-id>",
		Short: "Export sanitized CSV list of downloader subscribers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/export-downloaders", projectID), nil)
			if err != nil {
				return err
			}

			destPath, _ := cmd.Flags().GetString("output")
			if destPath == "" {
				destPath = fmt.Sprintf("downloaders-project-%s.csv", projectID)
			}

			if err := os.WriteFile(destPath, raw, 0644); err != nil {
				return fmt.Errorf("failed to save CSV file: %w", err)
			}

			a.printer.Info(fmt.Sprintf("✓ Exported %d bytes to %s", len(raw), destPath))
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Destination CSV file path")
	return cmd
}
