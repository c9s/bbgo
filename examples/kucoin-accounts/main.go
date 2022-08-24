package main

import (
	"context"
	"os"
	"strings"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
)

func init() {
	rootCmd.PersistentFlags().String("kucoin-api-key", "", "okex api key")
	rootCmd.PersistentFlags().String("kucoin-api-secret", "", "okex api secret")
	rootCmd.PersistentFlags().String("kucoin-api-passphrase", "", "okex api secret")
}

var rootCmd = &cobra.Command{
	Use:   "kucoin-accounts",
	Short: "kucoin accounts",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		req := client.AccountService.NewListAccountsRequest()
		accounts, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		log.Infof("accounts: %+v", accounts)
		return nil
	},
}

var client *kucoinapi.RestClient

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
