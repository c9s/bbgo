package cmd

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
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

		userConfig, err := bbgo.LoadBuildConfig(configFile)
		if err != nil {
			return err
		}

		if userConfig.Build == nil {
			return errors.New("build config is not defined")
		}

		for _, target := range userConfig.Build.Targets {
			log.Infof("building %s ...", target.Name)

			binary, err := bbgo.BuildTarget(ctx, userConfig, target)
			if err != nil {
				return err
			}

			log.Infof("build succeeded: %s", binary)
		}

		return nil
	},
}
