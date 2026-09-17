package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

// app holds the state every command shares once the root flags are parsed.
type app struct {
	cfg     *config.Config
	apiCli  *client.Client
	printer *output.Printer
	openURL func(url string)
	sleep   func(d time.Duration)
}

// NewRootCmd builds the full bookbeam command tree.
func NewRootCmd() *cobra.Command {
	a := &app{openURL: openBrowser, sleep: time.Sleep}
	var host, token string
	var jsonOutput, quiet bool

	root := &cobra.Command{
		Use:   "bookbeam",
		Short: "Official command-line interface for BookBeam (bookbeam.app)",
		Long: `BookBeam CLI lets authors and developers manage book projects, files,
signup links, newsletters, downloaders, and telemetry directly from the terminal.`,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.load(config.Load, host, token, output.New(jsonOutput, quiet))
		},
	}

	root.PersistentFlags().StringVar(&host, "host", "", "BookBeam API host (default https://bookbeam.app)")
	root.PersistentFlags().StringVar(&token, "token", "", "BookBeam API personal access token")
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress informational messages")

	root.AddCommand(
		authCmd(a),
		whoamiCmd(a),
		billingCmd(a),
		completionCmd(),
		downloadersCmd(a),
		filesCmd(a),
		linksCmd(a),
		newsletterCmd(a),
		projectsCmd(a),
		logsCmd(a),
		metricsCmd(a),
	)
	return root
}

// load resolves configuration, applying the --host and --token overrides.
func (a *app) load(loadConfig func(path string) (*config.Config, error), host, token string, printer *output.Printer) error {
	cfg, err := loadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if host != "" {
		cfg.Host = host
	}
	if token != "" {
		cfg.Token = token
	}

	a.cfg = cfg
	a.apiCli = client.New(cfg.Host, cfg.Token)
	a.printer = printer
	return nil
}

// confirm asks a yes/no question on the command's streams; anything but y/yes cancels.
func confirm(cmd *cobra.Command, question string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "%s (y/N): ", question)
	scanner := bufio.NewScanner(cmd.InOrStdin())
	scanner.Scan()
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// Execute runs the command tree and returns the error that ended it.
func Execute() error {
	return NewRootCmd().Execute()
}
