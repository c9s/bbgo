package okex

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the k-line stream logs with the logger given to the main stream, plus its endpoint
func TestStream_SetLogger_KLineStream(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	ex := New("", "", "")
	s := NewStream(ex.client, ex)
	s.SetLogger(logger.WithField("session", "okex"))

	require.NoError(t, s.kLineStream.StandardStream.Close())
	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "okex", entry.Data["session"])
	assert.Equal(t, "business", entry.Data["endpoint"])
	assert.Equal(t, "public", entry.Data["stream"])
}
