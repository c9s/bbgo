package cmd

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	BuildCmd.Flags().StringP("output", "o", "", "binary output")
	BuildCmd.Flags().String("os", runtime.GOOS, "GOOS")
	BuildCmd.Flags().String("arch", runtime.GOARCH, "GOARCH")
	BuildCmd.Flags().String("config", "", "config file")
	RootCmd.AddCommand(BuildCmd)
}

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "build cross-platform binary",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		output, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		userConfig, err := bbgo.Preload(configFile)
		if err != nil {
			return err
		}

		goOS, err := cmd.Flags().GetString("os")
		if err != nil {
			return err
		}

		goArch, err := cmd.Flags().GetString("arch")
		if err != nil {
			return err
		}

		buildDir := filepath.Join("build", "bbgow")

		binary, err := build(ctx, buildDir, userConfig, goOS, goArch, &output)
		if err != nil {
			return err
		}

		log.Infof("build succeeded: %s", binary)
		return nil
	},
}
