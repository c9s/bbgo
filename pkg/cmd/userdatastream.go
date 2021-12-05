package cmd

import (
	"context"
	"fmt"
	"syscall"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// go run ./cmd/bbgo userdatastream --session=ftx
var userDataStreamCmd = &cobra.Command{
	Use: "userdatastream",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if userConfig == nil {
			return errors.New("--config option or config file is missing")
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		s := session.Exchange.NewStream()

		s.OnOrderUpdate(func(order types.Order) {
			log.Infof("order update: %+v", order)
		})
		s.OnTradeUpdate(func(trade types.Trade) {
			log.Infof("trade update: %+v", trade)
		})
		s.OnBalanceUpdate(func(trade types.BalanceMap) {
			log.Infof("balance update: %+v", trade)
		})
		s.OnBalanceSnapshot(func(trade types.BalanceMap) {
			log.Infof("balance snapshot: %+v", trade)
		})
		s.OnPositionUpdate(func(position types.PositionMap) {
			log.Infof("position update: %+v", position)
		})
		s.OnPositionSnapshot(func(position types.PositionMap) {
			log.Infof("position snapshot: %+v", position)
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", sessionName)
		}
		log.Infof("connected")

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	userDataStreamCmd.Flags().String("session", "", "session name")
	RootCmd.AddCommand(userDataStreamCmd)
}
