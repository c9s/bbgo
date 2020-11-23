package cmdutil

import (
	"context"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
)

func WaitForSignal(ctx context.Context, signals ...os.Signal) os.Signal {
	var sigC = make(chan os.Signal, 1)
	signal.Notify(sigC, signals...)
	defer signal.Stop(sigC)

	select {
	case sig := <-sigC:
		logrus.Warnf("%v", sig)
		return sig

	case <-ctx.Done():
		return nil

	}

	return nil
}
