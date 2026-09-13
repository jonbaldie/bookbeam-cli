package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	flagDownloaderPage   int
	flagDownloaderOutput string
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

var downloadersCmd = &cobra.Command{
	Use:     "downloaders",
	Aliases: []string{"downloader"},
	Short:   "View reader downloaders and export subscriber lists",
}

var downloadersListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "List downloader subscribers for a book project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		query := url.Values{}
		if flagDownloaderPage > 0 {
			query.Set("page", strconv.Itoa(flagDownloaderPage))
		}

		raw, err := apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/downloaders", projectID), query)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var resp DownloaderListResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return err
		}

		if len(resp.Data) == 0 {
			printer.PrintInfo("No downloaders found for this project.")
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

		printer.Table(headers, rows)
		printer.PrintInfo(fmt.Sprintf("\nPage %d of %d (Total: %d)", resp.CurrentPage, resp.LastPage, resp.Total))
		return nil
	},
}

var downloadersExportCmd = &cobra.Command{
	Use:   "export <project-id>",
	Short: "Export sanitized CSV list of downloader subscribers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]

		raw, err := apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/export-downloaders", projectID), nil)
		if err != nil {
			return err
		}

		destPath := flagDownloaderOutput
		if destPath == "" {
			destPath = fmt.Sprintf("downloaders-project-%s.csv", projectID)
		}

		if err := os.WriteFile(destPath, raw, 0644); err != nil {
			return fmt.Errorf("failed to save CSV file: %w", err)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Exported %d bytes to %s", len(raw), destPath))
		return nil
	},
}

func init() {
	downloadersListCmd.Flags().IntVar(&flagDownloaderPage, "page", 1, "Page number")
	downloadersExportCmd.Flags().StringVarP(&flagDownloaderOutput, "output", "o", "", "Destination CSV file path")

	downloadersCmd.AddCommand(downloadersListCmd)
	downloadersCmd.AddCommand(downloadersExportCmd)

	rootCmd.AddCommand(downloadersCmd)
}
