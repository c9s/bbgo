// Command bars-backend is a standalone market-data server that serves regular
// klines and indicator/v2 tick/volume bars over REST + WebSocket for the
// bars-viewer frontend. It is independent of bbgo's own pkg/server.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/c9s/bbgo/cmd/bars-backend/internal/api"
	"github.com/c9s/bbgo/cmd/bars-backend/internal/config"
	"github.com/c9s/bbgo/cmd/bars-backend/internal/feed"
)

func main() {
	cfg := config.ParseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	f, err := feed.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize feed: %v", err)
	}

	log.Printf("backfilling and connecting to %s for symbols=%v intervals=%v", cfg.ExchangeName, cfg.Symbols, cfg.Intervals)
	if err := f.Start(ctx); err != nil {
		log.Fatalf("failed to start feed: %v", err)
	}

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: api.NewRouter(cfg, f),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	log.Printf("bars-backend listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
