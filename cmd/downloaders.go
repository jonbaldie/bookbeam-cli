package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

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

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			var resp DownloaderListResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				return err
			}

			if len(resp.Data) == 0 {
				a.printer.PrintInfo("No downloaders found for this project.")
				return nil
			}

			headers := []string{"EMAIL", "SIGNUP LINK", "SIGNUP DATE"}
			var rows [][]string
			for _, d := range resp.Data {
				rows = append(rows, []string{
					d.Email,
					d.LinkName,
					d.CreatedAt,
				})
			}

			a.printer.Table(headers, rows)
			a.printer.PrintInfo(fmt.Sprintf("\nPage %d of %d (Total: %d)", resp.CurrentPage, resp.LastPage, resp.Total))
			return nil
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

			a.printer.PrintInfo(fmt.Sprintf("✓ Exported %d bytes to %s", len(raw), destPath))
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Destination CSV file path")
	return cmd
}
