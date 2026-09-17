package main

import (
	"github.com/jonbaldie/bookbeam-cli/cmd"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(cmd.Execute())
}
