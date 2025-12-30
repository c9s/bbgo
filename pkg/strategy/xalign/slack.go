package xalign

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/google/uuid"
	"github.com/slack-go/slack"
)

var (
	_ slacknotifier.SlackBlocksCreator = &InteractiveSubmitOrder{}
)

const interactiveButtonsBlockID = "interactive_buttons"
const confirmOrderActionID = "confirm_order"
const cancelOrderActionID = "cancel_order"

var interactOrderRegistry sync.Map

type InteractiveSubmitOrder struct {
	sync.Mutex

	id          string
	submitOrder types.SubmitOrder
	delay       time.Duration
	mentions    []string

	slackEvtID string
	dueTime    time.Time
	done       bool

	cancelC  chan struct{}
	confirmC chan struct{}

	CancelOnce  func()
	ConfirmOnce func()
}

func NewInteractiveSubmitOrder(order types.SubmitOrder, delay time.Duration, mentions []string, slackEvtID string) *InteractiveSubmitOrder {
	itOrder := &InteractiveSubmitOrder{
		id:          uuid.NewString(),
		submitOrder: order,
		delay:       delay,
		mentions:    mentions,
		slackEvtID:  slackEvtID,

		cancelC:  make(chan struct{}),
		confirmC: make(chan struct{}),
	}
	itOrder.CancelOnce = sync.OnceFunc(func() { close(itOrder.cancelC) })
	itOrder.ConfirmOnce = sync.OnceFunc(func() { close(itOrder.confirmC) })
	interactOrderRegistry.Store(itOrder.id, itOrder)
	return itOrder
}

func (itOrder *InteractiveSubmitOrder) SlackBlocks() []slack.Block {
	// add the slack event id block
	// IMPORTANT: do not remove this block as it's used as a filter on upcoming interactions
	blocks := []slack.Block{
		slack.NewContextBlock(
			itOrder.slackEvtID,
			slack.NewTextBlockObject(
				slack.MarkdownType,
				"xalign Strategy Interactive Order",
				false,
				false,
			),
		),
	}
	dueTimeStr := "N/A"
	if !itOrder.dueTime.IsZero() {
		dueTimeStr = itOrder.dueTime.Format(time.RFC3339)
	}
	blocks = append(blocks, buildTextBlock(
		fmt.Sprintf(
			`:hammer: %s Order %s: %s %.4f@%.4f, quote amount %.4f
On %s after delay %s (due at %s)
- Confirm: immediately submit this order
- Cancel: immediately cancel this order`,
			itOrder.submitOrder.Type,
			itOrder.submitOrder.Side,
			itOrder.submitOrder.Symbol,
			itOrder.submitOrder.Quantity.Float64(),
			itOrder.submitOrder.Price.Float64(),
			itOrder.submitOrder.Price.Mul(itOrder.submitOrder.Quantity).Float64(),
			itOrder.submitOrder.Market.Exchange,
			itOrder.delay.String(),
			dueTimeStr,
		),
	))

	// add mention block
	if len(itOrder.mentions) > 0 {
		blocks = append(blocks, buildTextBlock(strings.Join(itOrder.mentions, " ")))
	}

	blocks = append(blocks, itOrder.buildButtonsBlock())
	return blocks
}

func (itOrder *InteractiveSubmitOrder) buildButtonsBlock() slack.Block {
	// all buttons' value should be itOrder.id
	return slack.NewActionBlock(
		interactiveButtonsBlockID,
		slack.NewButtonBlockElement(
			confirmOrderActionID,
			itOrder.id,
			slack.NewTextBlockObject(slack.PlainTextType, "Confirm", false, false),
		),
		slack.NewButtonBlockElement(
			cancelOrderActionID,
			itOrder.id,
			slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		))
}

// OnSubmittedOrderCallback is the callback function type on order submitted event
// Parameters:
// - session: the exchange session used to submit the order
// - submitOrder: the original submit order request
// - order: the created order, nil if error occurs
// - error: error if any during order submission
type OnSubmittedOrderCallback func(*bbgo.ExchangeSession, types.SubmitOrder, *types.Order, error)

func (itOrder *InteractiveSubmitOrder) AsyncSubmit(ctx context.Context, session *bbgo.ExchangeSession, onSubmittedOrderCallback OnSubmittedOrderCallback) {
	go itOrder.asyncSubmit(ctx, session, onSubmittedOrderCallback)
}

func (itOrder *InteractiveSubmitOrder) asyncSubmit(ctx context.Context, session *bbgo.ExchangeSession, onSubmittedOrderCallback OnSubmittedOrderCallback) {
	itOrder.Lock()
	defer itOrder.Unlock()

	// check done flag to avoid double submission
	if itOrder.done {
		return
	}

	if itOrder.dueTime.IsZero() {
		itOrder.dueTime = time.Now().Add(itOrder.delay)
		bbgo.Notify(itOrder) // send interactive order notification
	}

	// execute order submission
	timer := time.NewTimer(itOrder.delay)
	defer timer.Stop()

	var order *types.Order
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-itOrder.cancelC:
		err = fmt.Errorf("order is manually canceled: %v", itOrder.submitOrder)
	case <-itOrder.confirmC:
		order, err = session.Exchange.SubmitOrder(ctx, itOrder.submitOrder)
	case <-timer.C:
		order, err = session.Exchange.SubmitOrder(ctx, itOrder.submitOrder)
	}
	// mark as done
	itOrder.done = true
	// remove from registry
	interactOrderRegistry.Delete(itOrder.id)
	if onSubmittedOrderCallback != nil {
		onSubmittedOrderCallback(session, itOrder.submitOrder, order, err)
	}
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

func canceledSlackBlocks(userName string) []slack.Block {
	msgText := ""
	if userName == "" {
		msgText = ":x: Order Canceled"
	} else {
		msgText = ":x: Order Canceled by " + userName
	}
	msgText += ", " + time.Now().Format(time.RFC3339)
	return []slack.Block{
		buildTextBlock(msgText),
	}
}

func confirmedSlackBlocks(userName string) []slack.Block {
	msgText := ""
	if userName == "" {
		msgText = ":white_check_mark: Order Confirmed"
	} else {
		msgText = ":white_check_mark: Order Confirmed by " + userName
	}
	msgText += ", " + time.Now().Format(time.RFC3339)
	return []slack.Block{
		buildTextBlock(msgText),
	}
}

func cancelAllInteractiveOrders() {
	interactOrderRegistry.Range(func(key, value interface{}) bool {
		if io, ok := value.(*InteractiveSubmitOrder); ok {
			io.CancelOnce()
		}
		return true
	})
}

func setupSlackInteractionCallback(slackEvtID string) {
	for _, messenger := range interact.GetMessengers() {
		// setup the button click handler for the slack messenger
		slackMessenger, ok := messenger.(*interact.Slack)
		if !ok {
			continue
		}
		handler := func(user slack.User, oriMessage slack.Message, actionID string, actionValue string) ([]interact.InteractionMessageUpdate, error) {
			// check if the interactive order still exists
			value, ok := interactOrderRegistry.Load(actionValue)
			if !ok {
				// it's already processed
				return []interact.InteractionMessageUpdate{
					{
						Blocks:       removeBlockByID(oriMessage.Blocks.BlockSet, interactiveButtonsBlockID),
						Attachments:  nil,
						PostInThread: false,
					},
					{
						Blocks:       []slack.Block{buildTextBlock("*Order has been processed, cannot do any further actions.*")},
						Attachments:  nil,
						PostInThread: true,
					},
				}, nil
			}

			io, ok := value.(*InteractiveSubmitOrder)
			if !ok {
				// should not happen
				return []interact.InteractionMessageUpdate{
					{
						Blocks:       nil,
						Attachments:  nil,
						PostInThread: false,
					},
				}, fmt.Errorf("invalid type detected for interactive order (%s): %+v", actionValue, value)
			}

			blocks := removeBlockByID(oriMessage.Blocks.BlockSet, interactiveButtonsBlockID)
			switch actionID {
			case cancelOrderActionID:
				io.CancelOnce()
				blocks = append(blocks, canceledSlackBlocks(user.Name)...)
			case confirmOrderActionID:
				io.ConfirmOnce()
				blocks = append(blocks, confirmedSlackBlocks(user.Name)...)
			default:
				return []interact.InteractionMessageUpdate{
					{
						Blocks:       blocks,
						Attachments:  nil,
						PostInThread: false,
					},
					{
						Blocks:       []slack.Block{buildTextBlock("Invalid action ID: " + actionID)},
						Attachments:  nil,
						PostInThread: true,
					},
				}, nil
			}
			return []interact.InteractionMessageUpdate{
				{
					Blocks:       blocks,
					Attachments:  nil,
					PostInThread: false,
				},
			}, nil
		}
		dispatcher := interact.NewInteractiveMessageDispatcher(slackMessenger)
		dispatcher.Register(slackEvtID, handler)
	}
}
