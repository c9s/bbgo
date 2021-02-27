package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/types"
)

//godotenv -f .env.local go run ./cmd/bbgo balances --session=ftx
var balancesCmd = &cobra.Command{
	Use:          "balances",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		session, err := cmd.Flags().GetString("session")
		if err != nil {
			return fmt.Errorf("can't get session from flags: %w", err)
		}
		ex, err := newExchange(session)
		if err != nil {
			return err
		}
		b, err := ex.QueryAccountBalances(ctx)
		if err != nil {
			return err
		}
		log.Infof("balances: %+v", b)

		return nil
	},
}

func init() {
	balancesCmd.Flags().String("session", "", "the exchange session name for sync")

	RootCmd.AddCommand(balancesCmd)
}

func newExchange(session string) (types.Exchange, error) {
	switch session {
	case "ftx":
		return ftx.NewExchange(
			viper.GetString("ftx-api-key"),
			viper.GetString("ftx-api-secret"),
			viper.GetString("ftx-subaccount-name"),
		), nil

	}
	return nil, fmt.Errorf("unsupported session %s", session)
}
