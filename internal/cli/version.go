package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var VERSION = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the current version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(VERSION)

		return nil
	},
}
