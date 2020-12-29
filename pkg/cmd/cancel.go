package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

type advancedOrderCancelApi interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID string) ([]types.Order, error)
}

func init() {
	CancelCmd.Flags().String("session", "", "session to execute cancel orders")
	CancelCmd.Flags().String("symbol", "", "symbol to cancel orders")
	CancelCmd.Flags().String("groupID", "", "groupID to cancel orders")
	RootCmd.AddCommand(CancelCmd)
}

var CancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "cancel orders",
	Long:  "this command can cancel orders from exchange",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		groupID, err := cmd.Flags().GetString("groupID")
		if err != nil {
			return err
		}

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		userConfig, err := bbgo.Load(configFile, false)
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()

		if viper.IsSet("mysql-url") {
			db, err := cmdutil.ConnectMySQL()
			if err != nil {
				return err
			}
			environ.SetDB(db)
		}

		if err := environ.AddExchangesFromConfig(userConfig); err != nil {
			return err
		}

		if userConfig.Persistence != nil {
			if err := environ.ConfigurePersistence(userConfig.Persistence); err != nil {
				return err
			}
		}

		var sessions = environ.Sessions()

		if n, err := cmd.Flags().GetString("session"); err == nil && len(n) > 0 {
			ses, ok := sessions[n]
			if !ok {
				return fmt.Errorf("session %s not found", n)
			}

			sessions = map[string]*bbgo.ExchangeSession{n: ses}
		}

		for sessionID, session := range sessions {
			var log = logrus.WithField("session", sessionID)

			e, ok := session.Exchange.(advancedOrderCancelApi)
			if ok && len(groupID) > 0 {
				log.Infof("canceling orders by group id: %s", groupID)

				orders, err := e.CancelOrdersByGroupID(ctx, groupID)
				if err != nil {
					return err
				}

				for _, o := range orders {
					log.Info("CANCELED ", o.String())
				}

			} else if ok && len(symbol) > 0 {
				log.Infof("canceling orders by symbol: %s", symbol)

				orders, err := e.CancelOrdersBySymbol(ctx, symbol)
				if err != nil {
					return err
				}

				for _, o := range orders {
					log.Info("CANCELED ", o.String())
				}

			} else if len(symbol) > 0 {
				openOrders, err := session.Exchange.QueryOpenOrders(ctx, symbol)
				if err != nil {
					return err
				}

				if err := session.Exchange.CancelOrders(ctx, openOrders...); err != nil {
					return err
				}
			} else {
				log.Error("unsupported operation")
			}
		}

		return nil
	},
}
