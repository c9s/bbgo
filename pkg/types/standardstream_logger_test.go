package types

import (
	"context"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardStream_Kind(t *testing.T) {
	s := NewStandardStream()
	assert.Equal(t, "user_data", s.kind())

	s.SetPublicOnly()
	assert.Equal(t, "public", s.kind())
}

func TestStandardStream_LoggerFields(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	wsURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		defer c.Close()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetLogger(logger.WithField("exchange", "test"))
	s.SetName("futures_public")
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return wsURL, nil })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, s.Connect(ctx))
	require.NoError(t, s.Close())

	connected := findEntry(hook.AllEntries(), "[websocket] user data stream connected, read timeout = 2m0s")
	require.NotNil(t, connected, "no connected log entry in %v", messages(hook.AllEntries()))
	assert.Equal(t, "test", connected.Data["exchange"])
	assert.Equal(t, "user_data", connected.Data["stream"])
	assert.Equal(t, "futures_public", connected.Data["stream_name"])

	closing := findEntry(hook.AllEntries(), "[websocket] closing user data stream...")
	require.NotNil(t, closing, "no closing log entry in %v", messages(hook.AllEntries()))
	assert.Equal(t, "test", closing.Data["exchange"])
	assert.Equal(t, "user_data", closing.Data["stream"])
	assert.Equal(t, "futures_public", closing.Data["stream_name"])
}

// without SetLogger and SetName: the standard logger, the stream field, and no stream_name
func TestStandardStream_DefaultLogger(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	s := NewStandardStream()
	s.SetPublicOnly()
	s.getLogger().Info("hello")

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "public", entry.Data["stream"])
	assert.NotContains(t, entry.Data, "stream_name")
}

func findEntry(entries []*logrus.Entry, message string) *logrus.Entry {
	for _, e := range entries {
		if e.Message == message {
			return e
		}
	}
	return nil
}

func messages(entries []*logrus.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}
