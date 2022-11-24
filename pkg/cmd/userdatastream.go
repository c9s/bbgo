package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo userdatastream --session=binance
var userDataStreamCmd = &cobra.Command{
	Use:   "userdatastream",
	Short: "Listen to session events (orderUpdate, tradeUpdate, balanceUpdate, balanceSnapshot)",
	PreRunE: cobraInitRequired([]string{
		"session",
	}),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		s := session.Exchange.NewStream()
		s.OnOrderUpdate(func(order types.Order) {
			log.Infof("[orderUpdate] %+v", order)
		})
		s.OnTradeUpdate(func(trade types.Trade) {
			log.Infof("[tradeUpdate] %+v", trade)
		})
		s.OnBalanceUpdate(func(trade types.BalanceMap) {
			log.Infof("[balanceUpdate] %+v", trade)
		})
		s.OnBalanceSnapshot(func(trade types.BalanceMap) {
			log.Infof("[balanceSnapshot] %+v", trade)
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", sessionName)
		}

		log.Infof("connected")
		defer func() {
			log.Infof("closing connection...")
			if err := s.Close(); err != nil {
				log.WithError(err).Errorf("connection close error")
			}
			time.Sleep(1 * time.Second)
		}()

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	userDataStreamCmd.Flags().String("session", "", "session name")
	RootCmd.AddCommand(userDataStreamCmd)
}
