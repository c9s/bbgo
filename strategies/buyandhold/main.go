package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/buyandhold"
)

func init() {
	// persistent flags for trading environment
	bbgo.PersistentFlags(buyandhold.Cmd.PersistentFlags())
}

func main() {
	// general viper setup for trading environment
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	if err := viper.BindPFlags(buyandhold.Cmd.PersistentFlags()); err != nil {
		log.WithError(err).Errorf("failed to bind persistent flags. please check the flag settings.")
	}

	if err := buyandhold.Cmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}
