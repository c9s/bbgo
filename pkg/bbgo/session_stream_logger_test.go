package bbgo

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

func TestExchangeSession_setStreamLoggers(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	userDataStream := types.NewStandardStream()
	marketDataStream := types.NewStandardStream()
	marketDataStream.SetPublicOnly()

	session := &ExchangeSession{
		ExchangeSessionConfig: ExchangeSessionConfig{
			Name:         "binance_futures",
			ExchangeName: types.ExchangeBinance,
		},
		UserDataStream:   &userDataStream,
		MarketDataStream: &marketDataStream,
		logger:           logger.WithField("session", "binance_futures"),
	}
	session.setStreamLoggers()

	for stream, kind := range map[*types.StandardStream]string{
		&userDataStream:   "user_data",
		&marketDataStream: "public",
	} {
		hook.Reset()
		require.NoError(t, stream.Close())

		entry := hook.LastEntry()
		require.NotNil(t, entry)
		assert.Equal(t, "binance_futures", entry.Data["session"])
		assert.Equal(t, "binance", entry.Data["exchange"])
		assert.Equal(t, kind, entry.Data["stream"])
	}
}
