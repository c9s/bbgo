package main

import (
	"context"
	"os"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/exchange/binance"
)

func main() {
	log.SetLevel(log.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())

	// gobinance.NewClient(os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_API_SECRET"))

	ex := binance.New(os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_API_SECRET"))
	ex.UseMargin(true)
	stream := ex.NewStream()

	if err := stream.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
	cancel()
	time.Sleep(5 * time.Second)

	return
}
