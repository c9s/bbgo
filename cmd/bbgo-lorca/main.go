package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	time2 "time"

	"github.com/joho/godotenv"
	"github.com/zserge/lorca"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/server"
)

func main() {
	ep, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to find the current executable:", err)
	}

	err = os.Chdir(filepath.Join(filepath.Dir(ep), "..", "Resources"))
	if err != nil {
		log.Fatalln("chdir error:", err)
	}

	dotenvFile := ".env.local"
	if _, err := os.Stat(dotenvFile); err == nil {
		if err := godotenv.Load(dotenvFile); err != nil {
			log.WithError(err).Error("error loading dotenv file")
			return
		}
	}

	var args []string
	if runtime.GOOS == "linux" {
		args = append(args, "--class=bbgo")
	}

	// args = append(args, "--enable-logging", "--v=99")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// here allocate a chrome window with a blank page.
	ui, err := lorca.New("", "", 1024, 780, args...)
	if err != nil {
		log.WithError(err).Error("failed to initialize the window")
		return
	}

	defer ui.Close()

	configFile := "bbgo.yaml"
	var setup *server.Setup
	var userConfig *bbgo.Config

	_, err = os.Stat(configFile)
	if os.IsNotExist(err) {
		setup = &server.Setup{
			Context: ctx,
			Cancel:  cancel,
			Token:   "",
			BeforeRestart: func() {
				if err := ui.Close(); err != nil {
					log.WithError(err).Errorf("ui close error")
				}
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

		if err := trader.Configure(userConfig); err != nil {
			log.WithError(err).Error("failed to configure trader")
			return
		}

		if err := trader.LoadState(ctx); err != nil {
			log.WithError(err).Error("failed to load strategy states")
			return
		}

		// for setup mode, we don't start the trader
		go func() {
			if err := trader.Run(ctx); err != nil {
				log.WithError(err).Error("failed to start trader")
			}
		}()
	}

	// find a free port for binding the server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
	server.PingUntil(ctx, time2.Second, baseURL, func() {
		log.Infof("got pong, loading base url %s to ui...", baseURL)

		if err := ui.Load(baseURL); err != nil {
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
