package bbgo

import (
	"context"

	"github.com/pkg/errors"
)

func BootstrapEnvironment(ctx context.Context, environ *Environment, userConfig *Config) error {
	if err := environ.ConfigureDatabase(ctx); err != nil {
		return err
	}

	if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
		return errors.Wrap(err, "exchange session configure error")
	}

	if userConfig.Persistence != nil {
		if err := environ.ConfigurePersistence(userConfig.Persistence); err != nil {
			return errors.Wrap(err, "persistence configure error")
		}
	}

	if err := environ.ConfigureNotificationSystem(userConfig); err != nil {
		return errors.Wrap(err, "notification configure error")
	}

	return nil
}

func BootstrapBacktestEnvironment(ctx context.Context, environ *Environment) error {
	return environ.ConfigureDatabase(ctx)
}

