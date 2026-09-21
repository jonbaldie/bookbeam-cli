package cmd

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

type BookFileItem struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"book_project_id"`
	Filename  string `json:"filename"`
	Format    string `json:"file_type"`
	FileSize  int64  `json:"file_size"`
	Downloads int    `json:"downloads_count"`
	CreatedAt string `json:"created_at"`
}

func filesCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "files",
		Aliases: []string{"file"},
		Short:   "Manage book files (EPUB, MOBI, PDF) attached to projects",
	}
	cmd.AddCommand(filesListCmd(a), filesUploadCmd(a), filesDownloadCmd(a), filesDeleteCmd(a))
	return cmd
}

func filesListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list <project-id>",
		Short: "List all files attached to a book project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			raw, err := a.apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/files", projectID), nil)
			if err != nil {
				return err
			}

			var response struct {
				Data []BookFileItem `json:"data"`
			}
			doc, err := decodeResponse(raw, &response)
			if err != nil {
				return err
			}

			var rows [][]string
			for _, f := range response.Data {
				rows = append(rows, []string{
					strconv.Itoa(f.ID),
					f.Filename,
					strings.ToUpper(f.Format),
					strconv.FormatInt(f.FileSize, 10),
					strconv.Itoa(f.Downloads),
					f.CreatedAt,
				})
			}

			return a.printer.Display(output.View{
				Headers:     []string{"ID", "FILENAME", "FORMAT", "SIZE (BYTES)", "DOWNLOADS", "CREATED"},
				Rows:        rows,
				EmptyNotice: "No files attached to this project.",
				Data:        doc,
			})
		},
	}
}

func filesUploadCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "upload <project-id> <file-path>",
		Short: "Upload a book file (EPUB, MOBI, PDF) to a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			filePath := args[1]

			stat, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("file not found: %w", err)
			}

			ext := strings.ToLower(filepath.Ext(filePath))
			if ext != ".epub" && ext != ".mobi" && ext != ".pdf" {
				return fmt.Errorf("unsupported file extension '%s'; allowed extensions are .epub, .mobi, .pdf", ext)
			}

			a.printer.Info(fmt.Sprintf("Uploading %s (%d bytes) to project #%s...", filepath.Base(filePath), stat.Size(), projectID))

			raw, err := a.apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%s/files", projectID), nil, "file", filePath)
			if err != nil {
				return err
			}

			var response struct {
				Data BookFileItem `json:"data"`
			}
			doc, err := decodeResponse(raw, &response)
			if err != nil {
				return err
			}
			created := response.Data
			return a.printer.Success(doc, fmt.Sprintf("✓ Uploaded file #%d (%s) successfully.", created.ID, created.Filename))
		},
	}
}

func filesDownloadCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <project-id> <file-id>",
		Short: "Download a book file to your local disk",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			fileID := args[1]
			outputFlag, _ := cmd.Flags().GetString("output")

			resp, err := a.apiCli.Request(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/files/%s/download", projectID, fileID), nil, "")
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			destPath := resolveDownloadDestination(outputFlag, resp.Header.Get("Content-Disposition"), projectID, fileID)

			a.printer.Info(fmt.Sprintf("Downloading file to %s...", destPath))

			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}
			defer outFile.Close()

			n, err := io.Copy(outFile, resp.Body)
			if err != nil {
				return fmt.Errorf("failed to write file contents: %w", err)
			}

			a.printer.Info(fmt.Sprintf("✓ Download complete (%d bytes written to %s).", n, destPath))
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Destination file path")
	return cmd
}

func resolveDownloadDestination(outputFlag, dispositionHeader, projectID, fileID string) string {
	if outputFlag != "" {
		return outputFlag
	}

	if dispositionHeader != "" {
		if filename := extractDispositionFilename(dispositionHeader); filename != "" {
			return filename
		}
	}

	return fmt.Sprintf("project-%s-file-%s", projectID, fileID)
}

func extractDispositionFilename(header string) string {
	// A malformed header yields nil params, so the lookup falls through to "".
	_, params, _ := mime.ParseMediaType(header)
	return sanitizeFilename(params["filename"])
}

func sanitizeFilename(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	base := strings.TrimSpace(path.Base(cleaned))

	if base == "." || base == ".." || base == "/" {
		return ""
	}

	return base
}

func filesDeleteCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <project-id> <file-id>",
		Short: "Delete a book file from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			fileID := args[1]
			force, _ := cmd.Flags().GetBool("force")

			if !force && !a.printer.JSON {
				if !confirm(cmd, fmt.Sprintf("Are you sure you want to delete file #%s from project #%s?", fileID, projectID)) {
					a.printer.Info("Cancelled.")
					return nil
				}
			}

			raw, err := a.apiCli.Delete(fmt.Sprintf("/api/v1/projects/%s/files/%s", projectID, fileID))
			if err != nil {
				return err
			}

			return a.printer.Success(responseDocument(raw), fmt.Sprintf("✓ Deleted file #%s from project #%s", fileID, projectID))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
