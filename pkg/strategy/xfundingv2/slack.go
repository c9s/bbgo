package xfundingv2

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
)

var (
	_ slacknotifier.SlackBlocksCreator = &interactiveCloseRound{}
)

// buttonsBlockID is the block ID of the action block that holds the
// interactive buttons. It is removed/replaced after a click.
const buttonsBlockID = "xfundingv2_close_round_buttons"

// action IDs for the interactive close-round buttons.
const (
	closeRoundActionID   = "close_round"
	confirmCloseActionID = "confirm_close_round"
)

// interactiveCloseRound renders a periodic, block-based Slack message carrying a
// "Close Round" button for a Ready round. It implements
// slacknotifier.SlackBlocksCreator so bbgo.Notify emits it as a blocks message
// (interactive buttons require blocks, not attachments).
//
// There is no registry: the target round is carried in the button value (the
// round's spot symbol) and re-located from s.ActiveRounds under s.mu when the
// operator confirms. This is idempotent and leak-free under periodic re-emission.
type interactiveCloseRound struct {
	// slackEvtID is the dispatch key. It is rendered as a context block so the
	// dispatcher can route button clicks on this message to the handler
	// registered for this strategy instance. IMPORTANT: do not omit it.
	slackEvtID string

	// symbol is the round's spot symbol; used to look up the live round.
	symbol string

	// roundID is the round's unique ID. Together with symbol it is encoded into
	// the button value so a click only affects the exact round the notification
	// was attached to (a symbol may host a different round by the time an older,
	// periodically re-emitted message is clicked).
	roundID string

	spotPrice, futuresPrice fixedpoint.Value

	round *ArbitrageRound
}

func newInteractiveCloseRound(
	round *ArbitrageRound, slackEvtID string, spotPrice, futuresPrice fixedpoint.Value,
) *interactiveCloseRound {
	return &interactiveCloseRound{
		slackEvtID:   slackEvtID,
		symbol:       round.SpotSymbol(),
		roundID:      round.ID(),
		spotPrice:    spotPrice,
		futuresPrice: futuresPrice,
		round:        round,
	}
}

// closeRoundValueSep separates the symbol and round ID inside a button value.
// Neither a spot symbol nor a UUID round ID contains it.
const closeRoundValueSep = "||"

// encodeCloseRoundValue packs symbol and round ID into a single button value.
func encodeCloseRoundValue(symbol, roundID string) string {
	return symbol + closeRoundValueSep + roundID
}

// decodeCloseRoundValue splits a button value back into symbol and round ID.
func decodeCloseRoundValue(value string) (symbol, roundID string) {
	symbol, roundID, _ = strings.Cut(value, closeRoundValueSep)
	return symbol, roundID
}

func (c *interactiveCloseRound) SlackBlocks() []slack.Block {
	// The context block ID is the dispatch key. IMPORTANT: keep this block.
	blocks := []slack.Block{
		slack.NewContextBlock(
			c.slackEvtID,
			slack.NewTextBlockObject(
				slack.MarkdownType,
				"   ",
				false,
				false,
			),
		),
	}

	blocks = append(blocks, buildTextBlock(fmt.Sprintf(
		"🟢 Ready Round %s (%s)\nPress *Close Round* to close it.",
		c.symbol,
		c.roundID,
	)))

	blocks = append(blocks, buildCloseRoundButtonsBlock(c.symbol, c.roundID))
	return blocks
}

// buildCloseRoundButtonsBlock builds the initial single "Close Round" button.
// The button value carries the round's spot symbol and round ID so the handler
// can re-locate the exact live round on confirm.
func buildCloseRoundButtonsBlock(symbol, roundID string) slack.Block {
	return slack.NewActionBlock(
		buttonsBlockID,
		slack.NewButtonBlockElement(
			closeRoundActionID,
			encodeCloseRoundValue(symbol, roundID),
			slack.NewTextBlockObject(slack.PlainTextType, "Close Round", false, false),
		).WithStyle(slack.StyleDefault),
	)
}

// buildConfirmButtonsBlock builds the two-step confirmation buttons that
// replace the initial "Close Round" button after the first click.
func buildConfirmButtonsBlock(symbol, roundID string) slack.Block {
	value := encodeCloseRoundValue(symbol, roundID)
	return slack.NewActionBlock(
		buttonsBlockID,
		slack.NewButtonBlockElement(
			confirmCloseActionID,
			value,
			slack.NewTextBlockObject(slack.PlainTextType, "Confirm Close", false, false),
		).WithStyle(slack.StyleDanger),
	)
}

func buildTextBlock(text string) slack.Block {
	return slack.NewSectionBlock(
		slack.NewTextBlockObject(
			slack.MarkdownType,
			text,
			false,
			false,
		),
		nil, nil,
	)
}

func removeBlockByID(oriBlocks []slack.Block, id string) []slack.Block {
	var blocks []slack.Block
	for _, block := range oriBlocks {
		if block.ID() == id {
			continue // skip block
		}
		blocks = append(blocks, block)
	}
	return blocks
}
