//go:build !dnum

package xalign

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_InteractiveOrderSubmit(t *testing.T) {
	t.Run("single submission after delay", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx := context.Background()

		// Create a mock exchange
		mockEx := mocks.NewMockExchange(mockCtrl)

		// Create test market
		market := Market("BTCUSDT")
		market.Exchange = types.ExchangeMax

		// Create test submit order
		submitOrder := types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    Number(50000.0),
			Quantity: Number(0.001),
			Market:   market,
		}

		// Expected created order
		expectedOrder := types.Order{
			OrderID:          1,
			SubmitOrder:      submitOrder,
			ExecutedQuantity: fixedpoint.Zero,
			Status:           types.OrderStatusNew,
		}

		// Setup mock expectation
		mockEx.EXPECT().SubmitOrder(ctx, submitOrder).Return(&expectedOrder, nil).Times(1)

		// Create exchange session
		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
		}

		// Create interactive submit order with short delay
		delay := time.Second
		itOrder := NewInteractiveSubmitOrder(submitOrder, delay, nil, "")

		// Channel to capture callback result
		callbackCalled := make(chan struct{}, 1)
		var callbackOrder *types.Order
		var callbackErr error

		// async submit
		itOrder.AsyncSubmit(ctx, session, func(s *bbgo.ExchangeSession, so *types.SubmitOrder, order *types.Order, err error) {
			callbackOrder = order
			callbackErr = err
			callbackCalled <- struct{}{}
		})

		// Wait for callback to be called (with timeout)
		select {
		case <-callbackCalled:
			// Callback was called
			assert.NoError(t, callbackErr)
			assert.NotNil(t, callbackOrder)
			assert.Equal(t, uint64(1), callbackOrder.OrderID)
			assert.Equal(t, types.OrderStatusNew, callbackOrder.Status)
		case <-time.After(5 * time.Second):
			t.Fatal("submit callback was not called within timeout")
		}

		// Verify the order is removed from registry
		_, exists := interactOrderRegistry.Load(itOrder.id)
		assert.False(t, exists, "Order should be removed from registry after submission")
	})

	t.Run("concurrent submissions should only submit once", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx := context.Background()

		// Create a mock exchange
		mockEx := mocks.NewMockExchange(mockCtrl)

		// Create test market
		market := Market("BTCUSDT")
		market.Exchange = types.ExchangeMax

		// Create test submit order
		submitOrder := types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    Number(50000.0),
			Quantity: Number(0.001),
			Market:   market,
		}

		// Expected created order
		expectedOrder := types.Order{
			OrderID:          1,
			SubmitOrder:      submitOrder,
			ExecutedQuantity: fixedpoint.Zero,
			Status:           types.OrderStatusNew,
		}

		// Setup mock expectation - should only be called once
		mockEx.EXPECT().SubmitOrder(ctx, submitOrder).Return(&expectedOrder, nil).Times(1)

		// Create exchange session
		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
		}

		// Create interactive submit order with very short delay
		delay := 10 * time.Millisecond
		itOrder := NewInteractiveSubmitOrder(submitOrder, delay, nil, "")

		// Channels to capture callback results
		callbackCalled := make(chan struct{}, 1)
		var muCount sync.Mutex
		var callbackCount int = 0
		var callbackOrder *types.Order
		var callbackErr error

		// call AsyncSubmit twice
		var submitCallback OnSubmittedOrderCallback = func(s *bbgo.ExchangeSession, so *types.SubmitOrder, order *types.Order, err error) {
			muCount.Lock()
			defer muCount.Unlock()

			callbackOrder = order
			callbackErr = err
			callbackCount++
			close(callbackCalled)
		}
		itOrder.AsyncSubmit(ctx, session, submitCallback)
		itOrder.AsyncSubmit(ctx, session, submitCallback)

		select {
		case <-callbackCalled:
			// callback was called
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for callback")
		}

		// Exactly one callback should have been called with successful order submission
		assert.Equal(t, 1, callbackCount, "Only one callback should be called")
		assert.NoError(t, callbackErr)
		assert.Equal(t, uint64(1), callbackOrder.OrderID)
		assert.Equal(t, types.OrderStatusNew, callbackOrder.Status)

		// Verify the order is removed from registry
		_, exists := interactOrderRegistry.Load(itOrder.id)
		assert.False(t, exists, "Order should be removed from registry after submission")
	})
}

func Test_interact(t *testing.T) {
	// Create a mock Slack client
	slackClient := interact.NewSlack(
		slack.New("test-token"),
	)
	dispatcher := interact.NewInteractiveMessageDispatcher(slackClient)

	// register the button click handler
	setupSlackInteractionCallback("test-slack-evt-id", dispatcher)

	t.Run("cancel order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		// Create test market
		market := Market("BTCUSDT")
		market.Exchange = types.ExchangeMax

		// Create a test interactive submit order
		submitOrder := types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    Number(50000.0),
			Quantity: Number(0.001),
			Market:   market,
		}

		// Create interactive order with long delay so we can test cancellation
		itOrder := NewInteractiveSubmitOrder(submitOrder, 10*time.Second, nil, "")

		callbackCalled := make(chan struct{}, 1)
		// async submit
		ctx := context.Background()
		session := &bbgo.ExchangeSession{}
		var order *types.Order
		var err error
		itOrder.AsyncSubmit(ctx, session, func(s *bbgo.ExchangeSession, so *types.SubmitOrder, o *types.Order, e error) {
			order = o
			err = e
			callbackCalled <- struct{}{}
		})

		// Verify the order is in the registry
		_, exists := interactOrderRegistry.Load(itOrder.id)
		assert.True(t, exists, "Order should be in registry after creation")

		// Emit a button click event to cancel the order
		callback := slack.InteractionCallback{
			ActionCallback: slack.ActionCallbacks{
				BlockActions: []*slack.BlockAction{
					{
						ActionID: cancelOrderActionID,
						Value:    itOrder.id,
					},
				},
			},
			Message: slack.Message{
				Msg: slack.Msg{
					Blocks: slack.Blocks{
						BlockSet: []slack.Block{
							slack.NewContextBlock("test-slack-evt-id"),
							slack.NewActionBlock(interactiveButtonsBlockID),
						},
					},
					Timestamp: "123456.789",
				},
			},
			Channel: slack.Channel{
				GroupConversation: slack.GroupConversation{
					Conversation: slack.Conversation{
						ID: "test-channel",
					},
				},
			},
		}

		slackClient.EmitInteraction(callback)

		select {
		case <-callbackCalled:
			assert.True(t, itOrder.done)
			assert.Nil(t, order)
			assert.NotNil(t, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Cancel channel was not closed after button click")
		}
		_, exists = interactOrderRegistry.Load(itOrder.id)
		assert.False(t, exists, "Order should be removed from registry after cancellation")
	})

	t.Run("confirm order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx := context.Background()

		// Create a mock exchange
		mockEx := mocks.NewMockExchange(mockCtrl)

		// Create test market
		market := Market("BTCUSDT")
		market.Exchange = types.ExchangeMax

		// Create a test interactive submit order
		submitOrder := types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    Number(50000.0),
			Quantity: Number(0.001),
			Market:   market,
		}

		// Expected created order
		expectedOrder := types.Order{
			OrderID:          1,
			SubmitOrder:      submitOrder,
			ExecutedQuantity: fixedpoint.Zero,
			Status:           types.OrderStatusNew,
		}

		// Setup mock expectation
		mockEx.EXPECT().SubmitOrder(ctx, submitOrder).Return(&expectedOrder, nil).Times(1)

		// Create exchange session
		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
		}

		// Create interactive order with long delay so we can test immediate confirmation
		itOrder := NewInteractiveSubmitOrder(submitOrder, 10*time.Second, nil, "")

		// Verify the order is in the registry
		_, exists := interactOrderRegistry.Load(itOrder.id)
		assert.True(t, exists, "Order should be in registry after creation")

		// Channel to capture callback result
		callbackCalled := make(chan struct{}, 1)
		var callbackOrder *types.Order
		var callbackErr error

		// async submit
		itOrder.AsyncSubmit(ctx, session, func(s *bbgo.ExchangeSession, so *types.SubmitOrder, order *types.Order, err error) {
			callbackOrder = order
			callbackErr = err
			callbackCalled <- struct{}{}
		})

		// Emit a button click event to confirm the order
		callback := slack.InteractionCallback{
			ActionCallback: slack.ActionCallbacks{
				BlockActions: []*slack.BlockAction{
					{
						ActionID: confirmOrderActionID,
						Value:    itOrder.id,
					},
				},
			},
			Message: slack.Message{
				Msg: slack.Msg{
					Blocks: slack.Blocks{
						BlockSet: []slack.Block{
							slack.NewContextBlock("test-slack-evt-id"),
							slack.NewActionBlock(interactiveButtonsBlockID),
						},
					},
					Timestamp: "123456.789",
				},
			},
			Channel: slack.Channel{
				GroupConversation: slack.GroupConversation{
					Conversation: slack.Conversation{
						ID: "test-channel",
					},
				},
			},
		}

		slackClient.EmitInteraction(callback)

		// Wait for callback to be called (with timeout)
		select {
		case <-callbackCalled:
			assert.True(t, itOrder.done)
			assert.NoError(t, callbackErr)
			assert.NotNil(t, callbackOrder)
			assert.Equal(t, uint64(1), callbackOrder.OrderID)
			assert.Equal(t, types.OrderStatusNew, callbackOrder.Status)
		case <-time.After(5 * time.Second):
			t.Fatal("submit callback was not called within timeout")
		}

		_, exists = interactOrderRegistry.Load(itOrder.id)
		assert.False(t, exists, "Order should be removed from registry after submission")
	})
}
