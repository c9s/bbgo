package server

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

func pingUntil(ctx context.Context, baseURL string, callback func()) {
	pingURL := baseURL + "/api/ping"
	timeout := time.NewTimer(time.Minute)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-timeout.C:
			logrus.Warnf("ping hits 1 minute timeout")
			return

		case <-ctx.Done():
			return

		case <-ticker.C:
			var response map[string]interface{}
			var err = getJSON(pingURL, &response)
			if err == nil {
				go callback()
				return
			}
		}
	}
}

func pingAndOpenURL(ctx context.Context, baseURL string) {
	setupURL := baseURL + "/setup"
	go pingUntil(ctx, baseURL, func() {
		if err := openURL(setupURL); err != nil {
			logrus.WithError(err).Errorf("can not call open command to open the web page")
		}
	})
}
