package bbgo

import (
	"context"

	"github.com/pkg/errors"
)

// BootstrapEnvironmentLightweight bootstrap the environment in lightweight mode
// - no database configuration
// - no notification
func BootstrapEnvironmentLightweight(ctx context.Context, environ *Environment, userConfig *Config) error {
	if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
		return errors.Wrap(err, "exchange session configure error")
	}

	if userConfig.Persistence != nil {
		if err := ConfigurePersistence(ctx, userConfig.Persistence); err != nil {
			return errors.Wrap(err, "persistence configure error")
		}
	}

	return nil
}

func BootstrapEnvironment(ctx context.Context, environ *Environment, userConfig *Config) error {
	if err := environ.ConfigureDatabase(ctx); err != nil {
		return err
	}

	if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
		return errors.Wrap(err, "exchange session configure error")
	}

	if userConfig.Persistence != nil {
		if err := ConfigurePersistence(ctx, userConfig.Persistence); err != nil {
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
