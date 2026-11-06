package types

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardStream_Label(t *testing.T) {
	s := NewStandardStream()
	assert.Equal(t, "user data", s.label())

	s.SetPublicOnly()
	assert.Equal(t, "public", s.label())

	s.SetName("futures public")
	assert.Equal(t, "futures public", s.label())
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
	s.SetName("aux")
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return wsURL, nil })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, s.Connect(ctx))
	require.NoError(t, s.Close())

	connected := findEntry(hook.AllEntries(), "[websocket] aux stream connected, read timeout = 2m0s")
	require.NotNil(t, connected, "no connected log entry in %v", messages(hook.AllEntries()))
	assert.Equal(t, "test", connected.Data["exchange"])
	assert.Equal(t, "aux", connected.Data["stream"])

	closing := findEntry(hook.AllEntries(), "[websocket] closing aux stream...")
	require.NotNil(t, closing, "no closing log entry in %v", messages(hook.AllEntries()))
	assert.Equal(t, "test", closing.Data["exchange"])
	assert.Equal(t, "aux", closing.Data["stream"])
}

// without SetLogger and SetName the messages stay the same as before, plus the stream field
func TestStandardStream_DefaultLogger(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	s := NewStandardStream()
	s.SetPublicOnly()
	s.getLogger().Infof("[websocket] %s stream connected, read timeout = %v", s.label(), 2*time.Minute)

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "[websocket] public stream connected, read timeout = 2m0s", entry.Message)
	assert.Equal(t, "public", entry.Data["stream"])
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
