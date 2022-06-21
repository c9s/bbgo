package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(websocketCmd)
}

var websocketCmd = &cobra.Command{
	Use: "websocket",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	Args: cobra.ExactArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}

		var ctx = context.Background()
		var t = args[0]
		var err error
		var bullet *kucoinapi.Bullet

		switch t {
		case "public":
			bullet, err = client.BulletService.NewGetPublicBulletRequest().Do(ctx)
			if err != nil {
				return err
			}

			logrus.Infof("public bullet: %+v", bullet)

		case "private":
			bullet, err = client.BulletService.NewGetPrivateBulletRequest().Do(ctx)
			if err != nil {
				return err
			}

			logrus.Infof("private bullet: %+v", bullet)

		default:
			return errors.New("valid bullet type: public, private")

		}

		u, err := bullet.URL()
		if err != nil {
			return err
		}

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		logrus.Infof("connecting %s", u.String())
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			return err
		}

		defer c.Close()

		id := time.Now().UnixNano() / int64(time.Millisecond)
		wsCmds := []kucoin.WebSocketCommand{
			/*
				{
					Id:             id+1,
					Type:           "subscribe",
					Topic:          "/market/ticker:ETH-USDT",
					PrivateChannel: false,
					Response:       true,
				},
			*/
			{
				Id:             id + 2,
				Type:           "subscribe",
				Topic:          "/market/candles:ETH-USDT_1min",
				PrivateChannel: false,
				Response:       true,
			},
		}

		for _, wsCmd := range wsCmds {
			err = c.WriteJSON(wsCmd)
			if err != nil {
				return err
			}
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					logrus.Infoln("read:", err)
					return
				}

				logrus.Infof("recv: %s", message)
			}
		}()

		pingTicker := time.NewTicker(bullet.PingInterval())
		defer pingTicker.Stop()

		for {
			select {
			case <-done:
				return nil

			case <-pingTicker.C:
				if err := c.WriteJSON(kucoin.WebSocketCommand{
					Id:   time.Now().UnixNano() / int64(time.Millisecond),
					Type: "ping",
				}); err != nil {
					logrus.WithError(err).Error("websocket ping error", err)
				}

			case <-interrupt:
				logrus.Infof("interrupt")

				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					logrus.Error("write close:", err)
					return nil
				}

				select {
				case <-done:
				case <-time.After(time.Second):
				}
				return nil
			}
		}

		return nil
	},
}
