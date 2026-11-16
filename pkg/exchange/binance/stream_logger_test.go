package binance

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the side streams log with the logger given to the main stream, plus their endpoint
func TestStream_SetLogger_SideStreams(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	ex := New("", "")
	ex.UseFutures()
	s := NewStream(ex, ex.client, ex.futuresClient)
	s.SetLogger(logger.WithField("session", "binance_futures"))

	s.initFuturesAuxStream()
	require.NoError(t, s.futuresAuxStream.Close())
	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "binance_futures", entry.Data["session"])
	assert.Equal(t, "public", entry.Data["endpoint"])
	assert.Equal(t, "public", entry.Data["stream"])

	hook.Reset()
	statusStream := s.initFuturesAccountStatusStream()
	require.NoError(t, statusStream.Close())
	entry = hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "binance_futures", entry.Data["session"])
	assert.Equal(t, "ws_api", entry.Data["endpoint"])
	assert.Equal(t, "user_data", entry.Data["stream"])
}

// a stream built without SetLogger falls back to the package logger
func TestStream_SideStreamLogger_Default(t *testing.T) {
	s := &Stream{}
	entry, ok := s.sideStreamLogger("public").(*logrus.Entry)
	require.True(t, ok)
	assert.EqualValues(t, "binance", entry.Data["exchange"])
	assert.Equal(t, "public", entry.Data["endpoint"])
}
