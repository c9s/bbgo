package xmaker

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// recordOrderTimingUpdate feeds an order snapshot into tracker, routing to the terminal
// recording once the order leaves the active New/PartiallyFilled states so
// CancelWorked()/AdditionalFilledVolume() can be evaluated from the snapshot. Other terminal
// statuses (rejected, expired, ...) carry no cancel/fill timestamp semantics here, so they're
// left unrecorded.
func recordOrderTimingUpdate(tracker *types.OrderTimingTracker, order types.Order) {
	switch order.Status {
	case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
		tracker.RecordOrderUpdate(order)
	case types.OrderStatusCanceled:
		tracker.RecordServerCancelledOrder(order, order.UpdateTime.Time())
	case types.OrderStatusFilled:
		tracker.RecordServerFilledOrder(order, order.UpdateTime.Time())
	default:
	}
}

// recordSendCancelOrders records the "cancel sent" timestamp for every order about to be
// cancelled, right before the cancel request is issued.
func recordSendCancelOrders(tracker *types.OrderTimingTracker, sendAt time.Time, orders ...types.Order) {
	for _, o := range orders {
		tracker.RecordSendCancelOrder(o, sendAt)
	}
}
