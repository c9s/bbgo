package main

import (
	"context"
	"os"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.PersistentFlags().String("kucoin-api-key", "", "okex api key")
	rootCmd.PersistentFlags().String("kucoin-api-secret", "", "okex api secret")
	rootCmd.PersistentFlags().String("kucoin-api-passphrase", "", "okex api secret")

	rootCmd.AddCommand(accountsCmd)
	rootCmd.AddCommand(subAccountsCmd)
}

var rootCmd = &cobra.Command{
	Use:   "kucoin",
	Short: "kucoin",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var client *kucoinapi.RestClient = nil

func main() {
	if _, err := os.Stat(".env.local"); err == nil {
		if err := godotenv.Load(".env.local"); err != nil {
			log.Fatal(err)
		}
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Error("bind pflags error")
	}

	client = kucoinapi.NewClient()

	key, secret, passphrase := viper.GetString("kucoin-api-key"),
		viper.GetString("kucoin-api-secret"),
		viper.GetString("kucoin-api-passphrase")

	if len(key) == 0 || len(secret) == 0 || len(passphrase) == 0 {
		log.Fatal("empty key, secret or passphrase")
	}

	client.Auth(key, secret, passphrase)

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		log.WithError(err).Error("cmd error")
	}
}
