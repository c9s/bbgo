package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type advancedOrderCancelApi interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID int64) ([]types.Order, error)
}

func init() {
	cancelOrderCmd.Flags().String("session", "", "session to execute cancel orders")
	cancelOrderCmd.Flags().String("symbol", "", "symbol to cancel orders")
	cancelOrderCmd.Flags().Int64("group-id", 0, "group ID to cancel orders")
	cancelOrderCmd.Flags().Uint64("order-id", 0, "order ID to cancel orders")
	cancelOrderCmd.Flags().Bool("all", false, "cancel all orders")
	RootCmd.AddCommand(cancelOrderCmd)
}

var cancelOrderCmd = &cobra.Command{
	Use:   "cancel-order",
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

		groupID, err := cmd.Flags().GetInt64("group-id")
		if err != nil {
			return err
		}

		orderID, err := cmd.Flags().GetUint64("order-id")
		if err != nil {
			return err
		}

		all, err := cmd.Flags().GetBool("all")
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
		if err := environ.ConfigureDatabase(ctx); err != nil {
			return err
		}

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		if userConfig.Persistence != nil {
			if err := environ.ConfigurePersistence(userConfig.Persistence); err != nil {
				return err
			}
		}

		var sessions = environ.Sessions()

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		if len(sessionName) > 0 {
			ses, ok := sessions[sessionName]
			if !ok {
				return fmt.Errorf("session %s not found", sessionName)
			}

			if orderID > 0 {
				logrus.Infof("canceling order by the given order id %d", orderID)
				err := ses.Exchange.CancelOrders(ctx, types.Order{
					SubmitOrder: types.SubmitOrder{
						Symbol: symbol,
					},
					OrderID: orderID,
				})
				if err != nil {
					return err
				}
				return nil
			}

			sessions = map[string]*bbgo.ExchangeSession{sessionName: ses}
		}

		for sessionID, session := range sessions {
			var log = logrus.WithField("session", sessionID)

			e, ok := session.Exchange.(advancedOrderCancelApi)
			if ok {
				if all {
					log.Infof("canceling all orders")

					orders, err := e.CancelAllOrders(ctx)
					if err != nil {
						return err
					}

					for _, o := range orders {
						log.Info("CANCELED ", o.String())
					}
				} else if groupID > 0 {
					log.Infof("canceling orders by group id: %d", groupID)

					orders, err := e.CancelOrdersByGroupID(ctx, groupID)
					if err != nil {
						return err
					}

					for _, o := range orders {
						log.Info("CANCELED ", o.String())
					}
				} else if len(symbol) > 0 {
					log.Infof("canceling orders by symbol: %s", symbol)

					orders, err := e.CancelOrdersBySymbol(ctx, symbol)
					if err != nil {
						return err
					}

					for _, o := range orders {
						log.Info("CANCELED ", o.String())
					}
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
