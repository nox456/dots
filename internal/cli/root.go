package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = cobra.Command{
	Use:   "dots",
	Short: "A tool to manage dotfiles with symlinks",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
