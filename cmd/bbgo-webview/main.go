package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/webview/webview"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/server"
)

func main() {
	noChangeDir := false
	portNum := 0
	flag.BoolVar(&noChangeDir, "no-chdir", false, "do not change directory")
	flag.IntVar(&portNum, "port", 0, "server port")
	flag.Parse()

	if !noChangeDir {
		ep, err := os.Executable()
		if err != nil {
			log.Fatalln("failed to find the current executable:", err)
		}

		resourceDir := filepath.Join(filepath.Dir(ep), "..", "Resources")
		if _, err := os.Stat(resourceDir); err == nil {
			err = os.Chdir(resourceDir)
			if err != nil {
				log.Fatalln("chdir error:", err)
			}
		}
	}

	dotenvFile := ".env.local"
	if _, err := os.Stat(dotenvFile); err == nil {
		if err := godotenv.Load(dotenvFile); err != nil {
			log.WithError(err).Error("error loading dotenv file")
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	debug, _ := strconv.ParseBool(os.Getenv("DEBUG_WEBVIEW"))
	view := webview.New(debug)
	defer view.Destroy()

	view.SetTitle("BBGO")
	view.SetSize(1024, 780, webview.HintNone)

	configFile := "bbgo.yaml"
	var setup *server.Setup
	var userConfig *bbgo.Config

	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		setup = &server.Setup{
			Context: ctx,
			Cancel:  cancel,
			Token:   "",
			BeforeRestart: func() {
				view.Destroy()
			},
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
		if err := bbgo.BootstrapEnvironment(ctx, environ, userConfig); err != nil {
			log.WithError(err).Error("failed to bootstrap environment")
			return
		}

		// we could initialize the environment from the settings
		go func() {
			if err := environ.Sync(ctx); err != nil {
				log.WithError(err).Error("failed to sync data")
				return
			}

			if err := trader.Configure(userConfig); err != nil {
				log.WithError(err).Error("failed to configure trader")
				return
			}

			if err := trader.Initialize(ctx); err != nil {
				log.WithError(err).Error("failed to initialize strategies")
				return
			}

			if err := trader.LoadState(ctx); err != nil {
				log.WithError(err).Error("failed to load strategy states")
				return
			}

			// for setup mode, we don't start the trader
			if err := trader.Run(ctx); err != nil {
				log.WithError(err).Error("failed to start trader")
			}
		}()
	}

	// find a free port for binding the server
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(portNum))
	if err != nil {
		log.WithError(err).Error("can not bind listener")
		return
	}

	defer ln.Close()

	baseURL := "http://" + ln.Addr().String()

	srv := &server.Server{
		Config:        userConfig,
		Environ:       environ,
		Trader:        trader,
		OpenInBrowser: false,
		Setup:         setup,
	}

	go func() {
		if err := srv.RunWithListener(ctx, ln); err != nil {
			log.WithError(err).Errorf("server error")
		}
	}()

	log.Infof("pinging the server at %s", baseURL)
	server.PingUntil(ctx, time.Second, baseURL, func() {
		log.Infof("got pong, navigate to %s", baseURL)
		view.Navigate(baseURL)
		view.Run()
	})

	// Wait until the interrupt signal arrives or browser window is closed
	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)

	select {
	case <-sigc:
	}

	log.Println("exiting...")
}
