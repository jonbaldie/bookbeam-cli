package cmd

import (
	"errors"
	"net/http"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/spf13/cobra"
)

const publicCommand = "bookbeam.auth.public"

type authenticationError struct {
	cause error
}

func (*authenticationError) Error() string {
	return "Not authenticated. Run 'bookbeam auth login' to authenticate."
}

func (e *authenticationError) Unwrap() error {
	return e.cause
}

func requiresAuthentication(cmd *cobra.Command) bool {
	if cmd.Parent() == cmd.Root() && (cmd.Name() == "help" || cmd.Name() == cobra.ShellCompRequestCmd) {
		return false
	}
	return cmd.Annotations[publicCommand] != "true"
}

func (a *app) requireAuthentication(cmd *cobra.Command) error {
	if requiresAuthentication(cmd) && a.settings.Token() == "" {
		cmd.SilenceUsage = true
		return &authenticationError{}
	}
	return nil
}

func wrapAPIErrors(cmd *cobra.Command) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			err := run(cmd, args)
			var apiErr *client.APIError
			if errors.As(err, &apiErr) {
				cmd.SilenceUsage = true
				if apiErr.StatusCode == http.StatusUnauthorized {
					return &authenticationError{cause: err}
				}
			}
			return err
		}
	}
	for _, child := range cmd.Commands() {
		wrapAPIErrors(child)
	}
}
