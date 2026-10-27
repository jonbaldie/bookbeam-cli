package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/jonbaldie/bookbeam-cli/pkg/output"
	"github.com/spf13/cobra"
)

// app holds the state every command shares once the root flags are parsed.
type app struct {
	settings *config.Settings
	apiCli   *client.Client
	printer  *output.Printer
	openURL  func(url string)
	sleep    func(context.Context, time.Duration) error
}

// NewRootCmd builds the full bookbeam command tree over the process
// environment and home directory.
func NewRootCmd() *cobra.Command {
	return newRootCmd(os.Getenv, os.UserHomeDir)
}

// newRootCmd builds the command tree, resolving settings from getenv and home.
func newRootCmd(getenv func(string) string, home func() (string, error)) *cobra.Command {
	a := &app{openURL: openBrowser, sleep: waitContext}
	var host, token string
	var jsonOutput, quiet bool

	root := &cobra.Command{
		Use:   "bookbeam",
		Short: "Official command-line interface for BookBeam (bookbeam.app)",
		Long: `BookBeam CLI lets authors and developers manage book projects, files,
signup links, newsletters, downloaders, and telemetry directly from the terminal.`,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			overrides := config.Overrides{Host: host, Token: token}
			if err := a.load(getenv, home, overrides, output.New(jsonOutput, quiet)); err != nil {
				return err
			}
			return a.requireAuthentication(cmd)
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
	wrapAuthenticationErrors(root)
	return root
}

// load resolves settings, layering the --host and --token overrides on top.
func (a *app) load(getenv func(string) string, home func() (string, error), flags config.Overrides, printer *output.Printer) error {
	settings, err := resolveSettings(getenv, home, flags)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	a.settings = settings
	a.apiCli = client.New(settings.Host(), settings.Token())
	a.printer = printer
	return nil
}

func resolveSettings(getenv func(string) string, home func() (string, error), flags config.Overrides) (*config.Settings, error) {
	dir, err := config.Dir(getenv, home)
	if err != nil {
		return nil, err
	}
	return config.Resolve(dir, getenv, flags)
}

// Execute runs the command tree and returns the error that ended it.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return NewRootCmd().ExecuteContext(ctx)
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
