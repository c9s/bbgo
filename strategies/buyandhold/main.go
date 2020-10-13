package buyandhold

import (
	"context"
	"fmt"
	"syscall"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/buyandhold"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rootCmd.Flags().String("exchange", "", "target exchange")
	rootCmd.Flags().String("symbol", "", "trading symbol")
}

func connectMysql() (*sqlx.DB, error) {
	mysqlURL := viper.GetString("mysql-url")
	mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
	return sqlx.Connect("mysql", mysqlURL)
}

var rootCmd = &cobra.Command{
	Use:   "buyandhold",
	Short: "buy and hold",
	Long:  "hold trader",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		sessionID := "main"
		environ := bbgo.NewEnvironment(db)
		environ.AddExchange(sessionID, exchange).Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{})

		trader := bbgo.NewTrader(environ)
		trader.AttachStrategy(sessionID, buyandhold.New(symbol, "1h", 0.1))
		// trader.AttachCrossExchangeStrategy(...)
		err = trader.Run(ctx)

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}
