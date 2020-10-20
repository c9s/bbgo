package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/c9s/goose"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	MigrateCmd.Flags().Bool("no-update", false, "update source repository")
	RootCmd.AddCommand(MigrateCmd)
}

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "run database migration",
	SilenceUsage: true,
	Args:         cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		noUpdate, err := cmd.Flags().GetBool("no-update")
		if err != nil {
			return err
		}

		mysqlURL := viper.GetString("mysql-url")
		mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
		db, err := goose.OpenDBWithDriver("mysql", mysqlURL)
		if err != nil {
			return err
		}

		dotDir := bbgo.HomeDir()
		sourceDir := bbgo.SourceDir()
		migrationDir := path.Join(sourceDir, "migrations")

		log.Infof("creating dir: %s", dotDir)
		if err := os.Mkdir(dotDir, 0777); err != nil {
			// return err
		}

		log.Infof("checking %s", sourceDir)
		_, err = os.Stat(sourceDir)
		if err != nil {
			log.Infof("cloning bbgo source into %s ...", sourceDir)
			cmd := exec.CommandContext(ctx, "git", "clone", "https://github.com/c9s/bbgo", sourceDir)
			if err := cmd.Run(); err != nil {
				return err
			}
		} else if !noUpdate {
			log.Infof("updating: %s ...", sourceDir)
			cmd := exec.CommandContext(ctx, "git", "--work-tree", sourceDir, "pull")
			if err := cmd.Run(); err != nil {
				return err
			}
		}

		log.Infof("using migration file dir: %s", migrationDir)

		command := args[0]
		if err := goose.Run(command, db, migrationDir); err != nil {
			log.Fatalf("goose run: %v", err)
		}

		defer db.Close()

		return nil
	},
}
