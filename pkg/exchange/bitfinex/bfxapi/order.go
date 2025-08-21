package bfxapi

import (
	"encoding/json"
	"strings"
)

//go:generate mapgen -type OrderFlag
type OrderFlag int

const (
	OrderFlagHidden     OrderFlag = 64    // hidden order
	OrderFlagClose      OrderFlag = 512   // close position
	OrderFlagReduceOnly OrderFlag = 1024  // reduce-only
	OrderFlagPostOnly   OrderFlag = 4096  // post-only order
	OrderFlagOCO        OrderFlag = 16384 // one-cancels-other
	OrderFlagNoVarRate  OrderFlag = 524288
)

// OrderStatus represents the status of an order in Bitfinex.
// https://docs.bitfinex.com/docs/abbreviations-glossary#order-status
//
//go:generate mapgen -type OrderStatus
type OrderStatus string

const (
	OrderStatusActive            OrderStatus = "ACTIVE"             // order is active
	OrderStatusExecuted          OrderStatus = "EXECUTED"           // order has been fully filled
	OrderStatusPartiallyFilled   OrderStatus = "PARTIALLY FILLED"   // order has been partially filled
	OrderStatusCanceled          OrderStatus = "CANCELED"           // order has been canceled
	OrderStatusPostponed         OrderStatus = "POSTPONED"          // order has been postponed
	OrderStatusInsufficientBal   OrderStatus = "INSUFFICIENT BAL"   // insufficient balance
	OrderStatusNotEnoughBalance  OrderStatus = "NOT ENOUGH BALANCE" // not enough balance
	OrderStatusNotFound          OrderStatus = "NOT FOUND"          // order not found
	OrderStatusStopped           OrderStatus = "STOPPED"            // order stopped
	OrderStatusRejected          OrderStatus = "REJECTED"           // order rejected
	OrderStatusExpired           OrderStatus = "EXPIRED"            // order expired
	OrderStatusPending           OrderStatus = "PENDING"            // order pending
	OrderStatusPartiallyCanceled OrderStatus = "PARTIALLY CANCELED" // order partially canceled
)

// UnmarshalJSON implements custom unmarshaling for OrderStatus.
// It parses status strings like "EXECUTED @ 107.6(-0.2)", "CANCELED was: PARTIALLY FILLED @ ...", etc.
func (s *OrderStatus) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	raw = strings.ToUpper(raw)

	// direct match for known statuses
	switch {
	case strings.HasPrefix(raw, "ACTIVE"):
		*s = OrderStatusActive
	case strings.HasPrefix(raw, "EXECUTED") || strings.HasPrefix(raw, "FILLED"):
		*s = OrderStatusExecuted
	case strings.HasPrefix(raw, "PARTIALLY FILLED") || strings.HasPrefix(raw, "PARTIALLY EXECUTED"):
		*s = OrderStatusPartiallyFilled
	case strings.HasPrefix(raw, "CANCELED") || strings.HasPrefix(raw, "CANCELLED"):
		*s = OrderStatusCanceled
	case strings.HasPrefix(raw, "REJECTED"):
		*s = OrderStatusRejected
	case strings.HasPrefix(raw, "EXPIRED"):
		*s = OrderStatusExpired
	case strings.HasPrefix(raw, "INSUFFICIENT BAL") || strings.HasPrefix(raw, "NOT ENOUGH BALANCE") || strings.HasPrefix(raw, "INSUFFICIENT MARGIN"):
		*s = OrderStatusInsufficientBal
	case strings.HasPrefix(raw, "STOPPED"):
		*s = OrderStatusStopped
	case strings.HasPrefix(raw, "POSTPONED"):
		*s = OrderStatusPostponed
	case strings.HasPrefix(raw, "PENDING"):
		*s = OrderStatusPending
	case strings.HasPrefix(raw, "PARTIALLY CANCELED"):
		*s = OrderStatusPartiallyCanceled
	case strings.HasPrefix(raw, "NOT FOUND"):
		*s = OrderStatusNotFound
	case strings.HasPrefix(raw, "RSN_DUST") || strings.HasPrefix(raw, "RSN_PAUSE"):
		// treat as rejected
		*s = OrderStatusRejected
	default:
		// fallback: use the raw string
		*s = OrderStatus(raw)
	}

	return nil
}

//go:generate mapgen -type OrderType
type OrderType string

const (
	OrderTypeLimit         OrderType = "LIMIT"          // limit order
	OrderTypeExchangeLimit OrderType = "EXCHANGE LIMIT" // exchange limit order

	OrderTypeMarket               OrderType = "MARKET"                 // market order
	OrderTypeExchangeMarket       OrderType = "EXCHANGE MARKET"        // exchange market order
	OrderTypeStop                 OrderType = "STOP"                   // stop order
	OrderTypeExchangeStop         OrderType = "EXCHANGE STOP"          // exchange stop order
	OrderTypeStopLimit            OrderType = "STOP LIMIT"             // stop limit order
	OrderTypeExchangeStopLimit    OrderType = "EXCHANGE STOP LIMIT"    // exchange stop limit order
	OrderTypeTrailingStop         OrderType = "TRAILING STOP"          // trailing stop order
	OrderTypeExchangeTrailingStop OrderType = "EXCHANGE TRAILING STOP" // exchange trailing stop order
	OrderTypeFOK                  OrderType = "FOK"                    // fill-or-kill order
	OrderTypeExchangeFOK          OrderType = "EXCHANGE FOK"           // exchange fill-or-kill order
	OrderTypeIOC                  OrderType = "IOC"                    // immediate-or-cancel order
	OrderTypeExchangeIOC          OrderType = "EXCHANGE IOC"           // exchange immediate-or-cancel order
)
