package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Verbose bool
var Quiet bool
var DryRun bool
var Yes bool

var rootCmd = cobra.Command{
	Use:   "dots",
	Short: "A tool to manage dotfiles with symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Help()

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&Quiet, "quiet", false, "Quiet output")
	rootCmd.PersistentFlags().BoolVarP(&DryRun, "dry-run", "d", false, "Dry run")
	rootCmd.PersistentFlags().BoolVar(&Yes, "yes", false, "Answer yes to all questions")
}
