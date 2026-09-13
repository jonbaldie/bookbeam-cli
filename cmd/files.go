package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagFileOutput string
	flagFileForce  bool
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

var filesCmd = &cobra.Command{
	Use:     "files",
	Aliases: []string{"file"},
	Short:   "Manage book files (EPUB, MOBI, PDF) attached to projects",
}

var filesListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "List all files attached to a book project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		raw, err := apiCli.Get(fmt.Sprintf("/api/v1/projects/%s/files", projectID), nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var response struct {
			Data []BookFileItem `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return err
		}
		files := response.Data

		if len(files) == 0 {
			printer.PrintInfo("No files attached to this project.")
			return nil
		}

		headers := []string{"ID", "FILENAME", "FORMAT", "SIZE (BYTES)", "DOWNLOADS", "CREATED"}
		var rows [][]string
		for _, f := range files {
			rows = append(rows, []string{
				strconv.Itoa(f.ID),
				f.Filename,
				strings.ToUpper(f.Format),
				strconv.FormatInt(f.FileSize, 10),
				strconv.Itoa(f.Downloads),
				f.CreatedAt,
			})
		}

		printer.Table(headers, rows)
		return nil
	},
}

var filesUploadCmd = &cobra.Command{
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

		printer.PrintInfo(fmt.Sprintf("Uploading %s (%d bytes) to project #%s...", filepath.Base(filePath), stat.Size(), projectID))

		raw, err := apiCli.PostMultipart(fmt.Sprintf("/api/v1/projects/%s/files", projectID), nil, "file", filePath)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var response struct {
			Data BookFileItem `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return err
		}
		created := response.Data
		printer.PrintInfo(fmt.Sprintf("✓ Uploaded file #%d (%s) successfully.", created.ID, created.Filename))
		return nil
	},
}

var filesDownloadCmd = &cobra.Command{
	Use:   "download <project-id> <file-id>",
	Short: "Download a book file to your local disk",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		fileID := args[1]

		resp, err := apiCli.Request(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/files/%s/download", projectID, fileID), nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		destPath := flagFileOutput
		if destPath == "" {
			destPath = fmt.Sprintf("project-%s-file-%s", projectID, fileID)
		}

		printer.PrintInfo(fmt.Sprintf("Downloading file to %s...", destPath))

		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer outFile.Close()

		n, err := io.Copy(outFile, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write file contents: %w", err)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Download complete (%d bytes written to %s).", n, destPath))
		return nil
	},
}

var filesDeleteCmd = &cobra.Command{
	Use:   "delete <project-id> <file-id>",
	Short: "Delete a book file from a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		fileID := args[1]

		if !flagFileForce && !printer.JSON {
			fmt.Printf("Are you sure you want to delete file #%s from project #%s? (y/N): ", fileID, projectID)
			var ans string
			fmt.Scanln(&ans)
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "y" && ans != "yes" {
				printer.PrintInfo("Cancelled.")
				return nil
			}
		}

		raw, err := apiCli.Delete(fmt.Sprintf("/api/v1/projects/%s/files/%s", projectID, fileID))
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		printer.PrintInfo(fmt.Sprintf("✓ Deleted file #%s from project #%s", fileID, projectID))
		return nil
	},
}

func init() {
	filesDownloadCmd.Flags().StringVarP(&flagFileOutput, "output", "o", "", "Destination file path")
	filesDeleteCmd.Flags().BoolVarP(&flagFileForce, "force", "f", false, "Skip confirmation prompt")

	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDownloadCmd)
	filesCmd.AddCommand(filesDeleteCmd)

	rootCmd.AddCommand(filesCmd)
}
