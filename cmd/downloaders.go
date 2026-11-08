package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
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
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}
			query := url.Values{}
			page, _ := cmd.Flags().GetInt("page")
			if page > 0 {
				query.Set("page", strconv.Itoa(page))
			}

			raw, err := a.apiCli.GetWithContext(cmd.Context(), fmt.Sprintf("/api/v1/projects/%d/downloaders", projectID), query)
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

			emptyNotice := "No downloaders found for this project."
			keepFooter := false
			if resp.Total > 0 && (resp.CurrentPage > resp.LastPage || len(resp.Data) == 0) {
				emptyNotice = fmt.Sprintf("Page %d is past the last page.", resp.CurrentPage)
				keepFooter = true
			}

			return a.printer.Display(output.View{
				Headers:     []string{"EMAIL", "SIGNUP LINK", "SIGNUP DATE"},
				Rows:        rows,
				Footer:      fmt.Sprintf("\nPage %d of %d (Total: %d)", resp.CurrentPage, resp.LastPage, resp.Total),
				EmptyNotice: emptyNotice,
				KeepFooter:  keepFooter,
				Data:        doc,
			})
		},
	}
	cmd.Flags().Int("page", 1, "Page number")
	return cmd
}

// exportResult is the --json document for a completed CSV export.
type exportResult struct {
	ProjectID    int    `json:"project_id"`
	Path         string `json:"path"`
	BytesWritten int64  `json:"bytes_written"`
}

func downloadersExportCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <project-id>",
		Short: "Export sanitized CSV list of downloader subscribers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resourceID(args[0], "project")
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/api/v1/projects/%d/export-downloaders", projectID)
			destPath, _ := cmd.Flags().GetString("output")
			if destPath == "-" {
				return writeDownloaderCSVToStdout(a, cmd, path)
			}
			if destPath == "" {
				destPath = fmt.Sprintf("downloaders-project-%d.csv", projectID)
			}

			n, err := saveDownloaderCSV(cmd.Context(), a.apiCli, path, destPath)
			if err != nil {
				return err
			}
			result := exportResult{ProjectID: projectID, Path: destPath, BytesWritten: n}
			return a.printer.Success(result, fmt.Sprintf("✓ Exported %d bytes to %s", n, destPath))
		},
	}
	cmd.Flags().StringP("output", "o", "", "Destination CSV file path (use - for stdout)")
	return cmd
}

type csvWriter struct {
	io.Writer
	message string
}

func (w *csvWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return n, fmt.Errorf("%s: %w", w.message, err)
	}
	return n, nil
}

func writeDownloaderCSVToStdout(a *app, cmd *cobra.Command, path string) error {
	w := &csvWriter{Writer: a.printer.Out, message: "failed to write CSV to stdout"}
	_, n, err := client.StreamGet(a.apiCli, cmd.Context(), path, w)
	if err != nil {
		return err
	}
	if !a.printer.Quiet {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "✓ Exported %d bytes to stdout\n", n); err != nil {
			return fmt.Errorf("failed to write export confirmation to stderr: %w", err)
		}
	}
	return nil
}

func saveDownloaderCSV(ctx context.Context, api *client.Client, path, dest string) (int64, error) {
	file, err := createCSVTemp(dest)
	if err != nil {
		return 0, fmt.Errorf("failed to save CSV file: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	_, n, err := client.StreamGet(api, ctx, path, &csvWriter{Writer: file, message: "failed to save CSV file"})
	if err != nil {
		return 0, err
	}
	if err := file.Close(); err != nil {
		return 0, fmt.Errorf("failed to save CSV file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := os.Rename(file.Name(), dest); err != nil {
		return 0, fmt.Errorf("failed to save CSV file: %w", err)
	}
	return n, nil
}

func createCSVTemp(dest string) (*os.File, error) {
	info, err := os.Lstat(dest)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("destination is not a regular file")
	}
	mode := os.FileMode(0644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	name := filepath.Join(filepath.Dir(dest), fmt.Sprintf(".bookbeam-csv-%x", nonce))
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return file, nil
	}
	return preserveCSVMode(file, mode)
}

func preserveCSVMode(file *os.File, mode os.FileMode) (*os.File, error) {
	if err := file.Chmod(mode); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}
