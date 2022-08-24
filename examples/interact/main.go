package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/interact"
)

func parseFloatPercent(s string, bitSize int) (f float64, err error) {
	i := strings.Index(s, "%")
	if i < 0 {
		return strconv.ParseFloat(s, bitSize)
	}

	f, err = strconv.ParseFloat(s[:i], bitSize)
	if err != nil {
		return 0, err
	}
	return f / 100.0, nil
}

type closePositionTask struct {
	symbol     string
	percentage float64
	confirmed  bool
}

type positionInteraction struct {
	closePositionTask closePositionTask
}

// Commands implements the custom interaction
func (m *positionInteraction) Commands(i *interact.Interact) {
	i.Command("/closePosition", "", func(reply interact.Reply) error {
		// send symbol options
		reply.Message("Choose your position")
		for _, symbol := range []string{"BTCUSDT", "ETHUSDT"} {
			reply.AddButton(symbol, symbol, symbol)
		}

		return nil
	}).Next(func(reply interact.Reply, symbol string) error {
		// get symbol from user
		if len(symbol) == 0 {
			reply.Message("Please enter a symbol")
			return fmt.Errorf("empty symbol")
		}
		switch symbol {
		case "BTCUSDT", "ETHUSDT":

		default:
			reply.Message("Invalid symbol")
			return fmt.Errorf("invalid symbol")

		}

		m.closePositionTask.symbol = symbol

		reply.Message("Choose or enter the percentage to close")
		for _, symbol := range []string{"25%", "50%", "100%"} {
			reply.AddButton(symbol, symbol, symbol)
		}

		// send percentage options
		return nil
	}).Next(func(reply interact.Reply, percentageStr string) error {
		p, err := parseFloatPercent(percentageStr, 64)
		if err != nil {
			reply.Message("Not a valid percentage string")
			return err
		}

		// get percentage from user
		m.closePositionTask.percentage = p

		// send confirmation
		reply.Message("Are you sure to close the position?")
		reply.AddButton("Yes", "confirm", "yes")
		return nil
	}).Next(func(reply interact.Reply, confirm string) error {
		switch strings.ToLower(confirm) {
		case "yes":
			m.closePositionTask.confirmed = true
			reply.Message(fmt.Sprintf("Your %s position is closed", m.closePositionTask.symbol))

		default:

		}

		// call position close
		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		// reply result
		return nil
	})
}

func main() {
	log.SetFormatter(&prefixed.TextFormatter{})

	b, err := tb.NewBot(tb.Settings{
		// You can also set custom API URL.
		// If field is empty it equals to "https://api.telegram.org".
		// URL: "http://195.129.111.17:8012",

		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		// Synchronous: false,
		// Verbose: true,
		// ParseMode:   "",
		// Reporter:    nil,
		// Client:      nil,
		// Offline:     false,
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	ctx := context.Background()
	interact.AddMessenger(&interact.Telegram{
		Private: true,
		Bot:     b,
	})

	interact.AddCustomInteraction(&interact.AuthInteract{
		Strict: true,
		Mode:   interact.AuthModeToken,
		Token:  "123",
	})

	interact.AddCustomInteraction(&positionInteraction{})
	if err := interact.Start(ctx); err != nil {
		log.Fatal(err)
	}
	cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
}
