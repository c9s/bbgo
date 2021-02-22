package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/version"
)

func init() {
	// VersionCmd.Flags().String("session", "", "the exchange session name for sync")
	RootCmd.AddCommand(VersionCmd)
}

var VersionCmd = &cobra.Command{
	Use:          "version",
	Short:        "show version name",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Version)
	},
}

