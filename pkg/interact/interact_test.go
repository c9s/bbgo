package interact

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	tb "gopkg.in/tucnak/telebot.v2"
)

func Test_parseFuncArgsAndCall_NoErrorFunction(t *testing.T) {
	noErrorFunc := func(a string, b float64, c bool) error {
		assert.Equal(t, "BTCUSDT", a)
		assert.Equal(t, 0.123, b)
		assert.Equal(t, true, c)
		return nil
	}

	_, err := ParseFuncArgsAndCall(noErrorFunc, []string{"BTCUSDT", "0.123", "true"})
	assert.NoError(t, err)
}

func Test_parseFuncArgsAndCall_ErrorFunction(t *testing.T) {
	errorFunc := func(a string, b float64) error {
		return errors.New("error")
	}

	_, err := ParseFuncArgsAndCall(errorFunc, []string{"BTCUSDT", "0.123"})
	assert.Error(t, err)
}

func Test_parseFuncArgsAndCall_InterfaceInjection(t *testing.T) {
	f := func(w io.Writer, a string, b float64) error {
		_, err := w.Write([]byte("123"))
		return err
	}

	buf := bytes.NewBuffer(nil)
	_, err := ParseFuncArgsAndCall(f, []string{"BTCUSDT", "0.123"}, buf)
	assert.NoError(t, err)
	assert.Equal(t, "123", buf.String())
}

func Test_parseCommand(t *testing.T) {
	args := parseCommand(`closePosition "BTC USDT" 3.1415926 market`)
	t.Logf("args: %+v", args)
	for i, a := range args {
		t.Logf("args(%d): %#v", i, a)
	}

	assert.Equal(t, 4, len(args))
	assert.Equal(t, "closePosition", args[0])
	assert.Equal(t, "BTC USDT", args[1])
	assert.Equal(t, "3.1415926", args[2])
	assert.Equal(t, "market", args[3])
}

type closePositionTask struct {
	symbol     string
	percentage float64
	confirmed  bool
}

type TestInteraction struct {
	closePositionTask closePositionTask
}

func (m *TestInteraction) Commands(interact *Interact) {
	interact.Command("/closePosition", "", func(reply Reply) error {
		// send symbol options
		return nil
	}).Next(func(symbol string) error {
		// get symbol from user
		m.closePositionTask.symbol = symbol

		// send percentage options
		return nil
	}).Next(func(percentage float64) error {
		// get percentage from user
		m.closePositionTask.percentage = percentage

		// send confirmation
		return nil
	}).Next(func(confirmed bool) error {
		m.closePositionTask.confirmed = confirmed
		// call position close

		// reply result
		return nil
	})
}

func TestCustomInteraction(t *testing.T) {
	b, err := tb.NewBot(tb.Settings{
		Offline: true,
	})
	if !assert.NoError(t, err, "should have bot setup without error") {
		return
	}

	globalInteraction := New()

	telegram := &Telegram{
		Bot: b,
	}
	globalInteraction.AddMessenger(telegram)

	testInteraction := &TestInteraction{}
	testInteraction.Commands(globalInteraction)

	err = globalInteraction.init()
	assert.NoError(t, err)

	m := &tb.Message{
		Chat:   &tb.Chat{ID: 22},
		Sender: &tb.User{ID: 999},
	}
	session := telegram.loadSession(m)
	err = globalInteraction.runCommand(session, "/closePosition", []string{}, telegram.newReply(session))
	assert.NoError(t, err)

	assert.Equal(t, State("/closePosition_1"), session.CurrentState)

	err = globalInteraction.handleResponse(session, "BTCUSDT", telegram.newReply(session))
	assert.NoError(t, err)
	assert.Equal(t, State("/closePosition_2"), session.CurrentState)

	err = globalInteraction.handleResponse(session, "0.20", telegram.newReply(session))
	assert.NoError(t, err)
	assert.Equal(t, State("/closePosition_3"), session.CurrentState)

	err = globalInteraction.handleResponse(session, "true", telegram.newReply(session))
	assert.NoError(t, err)
	assert.Equal(t, State("public"), session.CurrentState)

	assert.Equal(t, closePositionTask{
		symbol:     "BTCUSDT",
		percentage: 0.2,
		confirmed:  true,
	}, testInteraction.closePositionTask)
}
