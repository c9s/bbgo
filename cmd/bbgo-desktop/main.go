package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/zserge/lorca"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd"
	"github.com/c9s/bbgo/pkg/server"
)

func main() {
	dotenvFile := ".env.local"

	if _, err := os.Stat(dotenvFile) ; err == nil {
		if err := godotenv.Load(dotenvFile); err != nil {
			log.WithError(err).Error("error loading dotenv file")
			return
		}
	}


	var args []string
	if runtime.GOOS == "linux" {
		args = append(args, "--class=bbgo")
	}
	args = append(args, "--class=bbgo")

	// here allocate a chrome window with a blank page.
	ui, err := lorca.New("", "", 800, 640, args...)
	if err != nil {
		log.WithError(err).Error("failed to initialize the window")
		return
	}

	defer ui.Close()

	// A simple way to know when UI is ready (uses body.onload event in JS)
	ui.Bind("start", func() {
		log.Println("lorca is ready")
	})

	// Create and bind Go object to the UI
	// ui.Bind("counterAdd", c.Add)

	// Load HTML.
	// You may also use `data:text/html,<base64>` approach to load initial HTML,
	// e.g: ui.Load("data:text/html," + url.PathEscape(html))
	// TODO: load the loading page html

	// find a free port for binding the server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.WithError(err).Error("can not bind listener")
		return
	}

	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configFile := "bbgo.yaml"
	var setup *server.Setup
	var userConfig *bbgo.Config

	_, err = os.Stat(configFile)
	if os.IsNotExist(err) {
		setup = &server.Setup{
			Context: ctx,
			Cancel:  cancel,
			Token:   "",
		}
		userConfig = &bbgo.Config{
			Notifications:      nil,
			Persistence:        nil,
			Sessions:           nil,
			ExchangeStrategies: nil,
		}
	} else {
		userConfig, err = bbgo.Load(configFile, true)
		if err != nil {
			log.WithError(err).Error("can not load config file")
			return
		}
	}

	environ := bbgo.NewEnvironment()
	trader := bbgo.NewTrader(environ)

	// we could initialize the environment from the settings
	if setup == nil {
		if err := cmd.BootstrapEnvironment(ctx, environ, userConfig) ; err != nil {
			log.WithError(err).Error("failed to bootstrap environment")
			return
		}

		if err := cmd.ConfigureTrader(trader, userConfig) ; err != nil {
			log.WithError(err).Error("failed to configure trader")
			return
		}

		// for setup mode, we don't start the trader
		trader.Subscribe()
		if err := trader.Run(ctx); err != nil {
			log.WithError(err).Error("failed to start trader")
			return
		}
	}

	go func() {
		srv := &server.Server{
			Config:        userConfig,
			Environ:       environ,
			Trader:        trader,
			OpenInBrowser: false,
			Setup: setup,
		}

		if err := srv.RunWithListener(ctx, ln); err != nil {
			log.WithError(err).Errorf("server error")
		}
	}()

	baseURL := "http://" + ln.Addr().String()

	go server.PingUntil(ctx, baseURL, func() {
		if err := ui.Load(baseURL) ; err != nil {
			log.WithError(err).Error("failed to load page")
		}
	})


	// Wait until the interrupt signal arrives or browser window is closed
	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)

	select {
	case <-sigc:
	case <-ui.Done():
	}

	log.Println("exiting...")
}
