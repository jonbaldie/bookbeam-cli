package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

// runCommand supports legacy handler-level tests, which bypass Cobra's Execute
// initialization. Cancellation behavior is tested through the real root.
func runCommand(command *cobra.Command, args []string) error {
	command.SetContext(context.Background())
	return command.RunE(command, args)
}
