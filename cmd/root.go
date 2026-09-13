package cmd

import (
	"fmt"
	"os"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	flagHost  string
	flagToken string
	flagJSON  bool
	flagQuiet bool

	cfg     *config.Config
	apiCli  *client.Client
	printer *output.Printer
)

var rootCmd = &cobra.Command{
	Use:   "bookbeam",
	Short: "Official command-line interface for BookBeam (bookbeam.app)",
	Long: `BookBeam CLI lets authors and developers manage book projects, files,
signup links, newsletters, downloaders, and telemetry directly from the terminal.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load("")
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if flagHost != "" {
			cfg.Host = flagHost
		}
		if flagToken != "" {
			cfg.Token = flagToken
		}

		apiCli = client.New(cfg.Host, cfg.Token)
		printer = output.New(flagJSON, flagQuiet)

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "BookBeam API host (default https://bookbeam.app)")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "BookBeam API personal access token")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output results as JSON")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress informational messages")
}
