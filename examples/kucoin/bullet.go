package main

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(bulletCmd)
}

var bulletCmd = &cobra.Command{
	Use: "bullet",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	Args: cobra.ExactArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}

		ctx := context.Background()
		t := args[0]

		switch t {
		case "public":
			bullet, err := client.BulletService.NewGetPublicBulletRequest().Do(ctx)
			if err != nil {
				return err
			}

			logrus.Infof("public bullet: %+v", bullet)

		case "private":
			bullet, err := client.BulletService.NewGetPrivateBulletRequest().Do(ctx)
			if err != nil {
				return err
			}

			logrus.Infof("private bullet: %+v", bullet)

		default:
			return errors.New("valid bullet type: public, private")

		}

		return nil
	},
}
