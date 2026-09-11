package xfundingv2

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

// blockIDs returns the IDs of the given blocks, for assertions.
func blockIDs(blocks []slack.Block) []string {
	ids := make([]string, 0, len(blocks))
	for _, b := range blocks {
		ids = append(ids, b.ID())
	}
	return ids
}

// hasBlockID reports whether any block has the given ID.
func hasBlockID(blocks []slack.Block, id string) bool {
	for _, got := range blockIDs(blocks) {
		if got == id {
			return true
		}
	}
	return false
}

// closeRoundMessage builds a slack.Message carrying the interactive close-round
// blocks, as it would appear when a click callback arrives.
func closeRoundMessage(c *interactiveCloseRound) slack.Message {
	return slack.Message{
		Msg: slack.Msg{
			Blocks:    slack.Blocks{BlockSet: c.SlackBlocks()},
			Timestamp: "123456.789",
		},
	}
}

// newStrategyFixture builds a Strategy fixture for testing
func newStrategyFixture(
	t *testing.T, ctrl *gomock.Controller, symbol, slackEvtID string,
) (*Strategy, *ArbitrageRound) {
	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
	round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
	round.syncState.State = RoundReady

	s := &Strategy{
		slackEvtID:        slackEvtID,
		ActiveRounds:      map[string]*ArbitrageRound{symbol: round},
		spotLastPrices:    map[string]fixedpoint.Value{symbol: Number(50000.0)},
		futuresMarkPrices: map[string]fixedpoint.Value{symbol: Number(50010.0)},
	}
	s.TWAPWorkerConfig.ClosingDuration = types.Duration(time.Hour)
	return s, round
}

// TestNotifyInteractiveCloseRound builds an interactive close-round notification
// and sends it through bbgo.Notify.
func TestNotifyInteractiveCloseRound(t *testing.T) {
	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	slackAppToken := os.Getenv("SLACK_APP_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if slackBotToken == "" || slackAppToken == "" || channel == "" {
		t.Skip("SLACK_BOT_TOKEN, SLACK_APP_TOKEN and SLACK_CHANNEL must be set")
	}
	if !strings.HasPrefix(slackBotToken, "xoxb-") {
		t.Fatal("SLACK_BOT_TOKEN must have the prefix \"xoxb-\"")
	}
	if !strings.HasPrefix(slackAppToken, "xapp-") {
		t.Fatal("SLACK_APP_TOKEN must have the prefix \"xapp-\"")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// build a live strategy fixture
	s, round := newStrategyFixture(t, ctrl, "BTCUSDT", "test-slack-evt-id")

	// wire a real Slack notifier into bbgo's notification hub
	client := slack.New(slackBotToken, slack.OptionAppLevelToken(slackAppToken))
	bbgo.Notification.AddNotifier(slacknotifier.New(client, channel))

	// wire the interactive button handler
	messenger := interact.NewSlack(client)
	dispatcher := interact.NewInteractiveMessageDispatcher(messenger)
	setupCloseRoundInteraction(s, dispatcher)

	go messenger.Start(context.Background())

	spotPrice, futuresPrice, _ := s.getLastPrices("BTCUSDT", "BTCUSDT")
	c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
	bbgo.Notify("Active Rounds", round.NewNotification(spotPrice, futuresPrice), c)

	// the Slack notifier posts asynchronously and socket mode delivers clicks on
	// its own goroutine; keep the process alive so the handler can run.
	for {
		time.Sleep(2 * time.Second)
	}
}

func TestCloseRoundInteraction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const symbol = "BTCUSDT"
	const slackEvtID = "test-slack-evt-id"

	t.Run("SlackBlocks include dispatch key and buttons", func(t *testing.T) {
		s, round := newStrategyFixture(t, ctrl, symbol, slackEvtID)
		c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
		blocks := c.SlackBlocks()

		assert.True(t, hasBlockID(blocks, slackEvtID), "must carry the slackEvtID context block")
		assert.True(t, hasBlockID(blocks, buttonsBlockID), "must carry the buttons block")
	})

	t.Run("Close reveals confirm/cancel without changing state", func(t *testing.T) {
		s, round := newStrategyFixture(t, ctrl, symbol, slackEvtID)
		c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
		handler := newCloseRoundHandler(s)

		value := encodeCloseRoundValue(symbol, round.ID())
		updates, err := handler(slack.User{Name: "alice"}, closeRoundMessage(c), closeRoundActionID, value)
		assert.NoError(t, err)
		assert.Len(t, updates, 1)
		// buttons block is replaced (same block ID), and no state change yet
		assert.True(t, hasBlockID(updates[0].Blocks, buttonsBlockID))
		assert.Equal(t, RoundReady, round.State(), "state must not change on first click")
	})

	t.Run("Confirm drives the round to closing", func(t *testing.T) {
		s, round := newStrategyFixture(t, ctrl, symbol, slackEvtID)
		c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
		handler := newCloseRoundHandler(s)

		value := encodeCloseRoundValue(symbol, round.ID())
		updates, err := handler(slack.User{Name: "alice"}, closeRoundMessage(c), confirmCloseActionID, value)
		assert.NoError(t, err)
		// confirm produces a single update replacing the message with the acknowledgement
		assert.Len(t, updates, 1)
		// the buttons block is stripped after confirm
		assert.False(t, hasBlockID(updates[0].Blocks, buttonsBlockID))
		assert.Equal(t, RoundClosing, round.State(), "confirm must set the round to closing")
	})

	t.Run("Confirm on unknown symbol is a no-op", func(t *testing.T) {
		s, round := newStrategyFixture(t, ctrl, symbol, slackEvtID)
		c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
		handler := newCloseRoundHandler(s)

		value := encodeCloseRoundValue("DOGEUSDT", round.ID())
		updates, err := handler(slack.User{Name: "alice"}, closeRoundMessage(c), confirmCloseActionID, value)
		assert.NoError(t, err)
		// no-op still emits the main update plus a threaded "not closeable" reply
		assert.Len(t, updates, 2)
		// original round is untouched
		assert.Equal(t, RoundReady, round.State())
	})

	t.Run("Confirm with a stale round ID is a no-op", func(t *testing.T) {
		s, round := newStrategyFixture(t, ctrl, symbol, slackEvtID)
		c := newInteractiveCloseRound(round, s.slackEvtID, Number(50000.0), Number(50010.0))
		handler := newCloseRoundHandler(s)

		// same symbol, but the notification points at a round ID that is no longer
		// the live round under this symbol — the click must not close it.
		value := encodeCloseRoundValue(symbol, "some-other-round-id")
		updates, err := handler(slack.User{Name: "alice"}, closeRoundMessage(c), confirmCloseActionID, value)
		assert.NoError(t, err)
		// no-op still emits the main update plus a threaded "not closeable" reply
		assert.Len(t, updates, 2)
		assert.Equal(t, RoundReady, round.State(), "a stale round ID must not close the live round")
		assert.False(t, hasBlockID(updates[0].Blocks, buttonsBlockID))
	})
}
