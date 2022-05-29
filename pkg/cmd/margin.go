package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

var selectedSession *bbgo.ExchangeSession

func init() {
	marginLoansCmd.Flags().String("asset", "", "asset")
	marginLoansCmd.Flags().String("session", "", "exchange session name")
	marginCmd.AddCommand(marginLoansCmd)

	marginRepaysCmd.Flags().String("asset", "", "asset")
	marginRepaysCmd.Flags().String("session", "", "exchange session name")
	marginCmd.AddCommand(marginRepaysCmd)

	marginInterestsCmd.Flags().String("asset", "", "asset")
	marginInterestsCmd.Flags().String("session", "", "exchange session name")
	marginCmd.AddCommand(marginInterestsCmd)

	RootCmd.AddCommand(marginCmd)
}

// go run ./cmd/bbgo margin --session=binance
var marginCmd = &cobra.Command{
	Use:          "margin",
	Short:        "margin related history",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := cobraLoadDotenv(cmd, args); err != nil {
			return err
		}

		if err := cobraLoadConfig(cmd, args); err != nil {
			return err
		}

		// ctx := context.Background()
		environ := bbgo.NewEnvironment()

		if userConfig == nil {
			return errors.New("user config is not loaded")
		}

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

		selectedSession = session
		return nil
	},
}

// go run ./cmd/bbgo margin loans --session=binance
var marginLoansCmd = &cobra.Command{
	Use:          "loans --session=SESSION_NAME --asset=ASSET",
	Short:        "query loans history",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return err
		}

		if selectedSession == nil {
			return errors.New("session is not set")
		}

		marginHistoryService, ok := selectedSession.Exchange.(types.MarginHistory)
		if !ok {
			return fmt.Errorf("exchange %s does not support MarginHistory service", selectedSession.ExchangeName)
		}

		now := time.Now()
		startTime := now.AddDate(0, -5, 0)
		endTime := now
		loans, err := marginHistoryService.QueryLoanHistory(ctx, asset, &startTime, &endTime)
		if err != nil {
			return err
		}

		log.Infof("%d loans", len(loans))
		for _, loan := range loans {
			log.Infof("LOAN %+v", loan)
		}

		return nil
	},
}

// go run ./cmd/bbgo margin loans --session=binance
var marginRepaysCmd = &cobra.Command{
	Use:          "repays --session=SESSION_NAME --asset=ASSET",
	Short:        "query repay history",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return err
		}

		if selectedSession == nil {
			return errors.New("session is not set")
		}

		marginHistoryService, ok := selectedSession.Exchange.(types.MarginHistory)
		if !ok {
			return fmt.Errorf("exchange %s does not support MarginHistory service", selectedSession.ExchangeName)
		}

		now := time.Now()
		startTime := now.AddDate(0, -5, 0)
		endTime := now
		repays, err := marginHistoryService.QueryLoanHistory(ctx, asset, &startTime, &endTime)
		if err != nil {
			return err
		}

		log.Infof("%d repays", len(repays))
		for _, repay := range repays {
			log.Infof("REPAY %+v", repay)
		}

		return nil
	},
}

// go run ./cmd/bbgo margin interests --session=binance
var marginInterestsCmd = &cobra.Command{
	Use:          "interests --session=SESSION_NAME --asset=ASSET",
	Short:        "query interests history",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return fmt.Errorf("can't get the symbol from flags: %w", err)
		}

		if selectedSession == nil {
			return errors.New("session is not set")
		}

		marginHistoryService, ok := selectedSession.Exchange.(types.MarginHistory)
		if !ok {
			return fmt.Errorf("exchange %s does not support MarginHistory service", selectedSession.ExchangeName)
		}

		now := time.Now()
		startTime := now.AddDate(0, -5, 0)
		endTime := now
		interests, err := marginHistoryService.QueryInterestHistory(ctx, asset, &startTime, &endTime)
		if err != nil {
			return err
		}

		log.Infof("%d interests", len(interests))
		for _, interest := range interests {
			log.Infof("INTEREST %+v", interest)
		}

		return nil
	},
}
