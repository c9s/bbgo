package bybit

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WsEvent struct {
	// "op" and "topic" are exclusive.
	*WebSocketOpEvent
	*WebSocketTopicEvent
}

func (w *WsEvent) IsOp() bool {
	return w.WebSocketOpEvent != nil && w.WebSocketTopicEvent == nil
}

func (w *WsEvent) IsTopic() bool {
	return w.WebSocketOpEvent == nil && w.WebSocketTopicEvent != nil
}

type WsOpType string

const (
	WsOpTypePing      WsOpType = "ping"
	WsOpTypePong      WsOpType = "pong"
	WsOpTypeAuth      WsOpType = "auth"
	WsOpTypeSubscribe WsOpType = "subscribe"
)

type WebsocketOp struct {
	Op   WsOpType `json:"op"`
	Args []string `json:"args"`
}

type WebSocketOpEvent struct {
	Success bool   `json:"success"`
	RetMsg  string `json:"ret_msg"`
	ReqId   string `json:"req_id,omitempty"`

	ConnId string   `json:"conn_id"`
	Op     WsOpType `json:"op"`
	Args   []string `json:"args"`
}

func (w *WebSocketOpEvent) IsValid() error {
	switch w.Op {
	case WsOpTypePing:
		// public event
		if !w.Success || WsOpType(w.RetMsg) != WsOpTypePong {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypePong:
		// private event, no success and ret_msg fields in response
		return nil
	case WsOpTypeAuth:
		if !w.Success || w.RetMsg != "" {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypeSubscribe:
		// in the public channel, you can get RetMsg = 'subscribe', but in the private channel, you cannot.
		// so, we only verify that success is true.
		if !w.Success {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	default:
		return fmt.Errorf("unexpected op type: %+v", w)
	}
}

type TopicType string

const (
	TopicTypeOrderBook TopicType = "orderbook"
	TopicTypeWallet    TopicType = "wallet"
)

type DataType string

const (
	DataTypeSnapshot DataType = "snapshot"
	DataTypeDelta    DataType = "delta"
)

type WebSocketTopicEvent struct {
	Topic string   `json:"topic"`
	Type  DataType `json:"type"`
	// The timestamp (ms) that the system generates the data
	Ts   types.MillisecondTimestamp `json:"ts"`
	Data json.RawMessage            `json:"data"`
}

type BookEvent struct {
	// Symbol name
	Symbol string `json:"s"`
	// Bids. For snapshot stream, the element is sorted by price in descending order
	Bids types.PriceVolumeSlice `json:"b"`
	// Asks. For snapshot stream, the element is sorted by price in ascending order
	Asks types.PriceVolumeSlice `json:"a"`
	// Update ID. Is a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of
	// the service. So please overwrite your local orderbook
	UpdateId fixedpoint.Value `json:"u"`
	// Cross sequence. You can use this field to compare different levels orderbook data, and for the smaller seq,
	// then it means the data is generated earlier.
	SequenceId fixedpoint.Value `json:"seq"`

	// internal use
	// Type can be one of snapshot or delta. Copied from WebSocketTopicEvent.Type
	Type DataType
}

func (e *BookEvent) OrderBook() (snapshot types.SliceOrderBook) {
	snapshot.Symbol = e.Symbol
	snapshot.Bids = e.Bids
	snapshot.Asks = e.Asks
	return snapshot
}

const topicSeparator = "."

func genTopic(in ...interface{}) string {
	out := make([]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	return strings.Join(out, topicSeparator)
}

func getTopicType(topic string) TopicType {
	slice := strings.Split(topic, topicSeparator)
	if len(slice) == 0 {
		return ""
	}
	return TopicType(slice[0])
}

type AccountType string

const AccountTypeSpot AccountType = "SPOT"

type WalletEvent struct {
	AccountType            AccountType      `json:"accountType"`
	AccountIMRate          fixedpoint.Value `json:"accountIMRate"`
	AccountMMRate          fixedpoint.Value `json:"accountMMRate"`
	TotalEquity            fixedpoint.Value `json:"totalEquity"`
	TotalWalletBalance     fixedpoint.Value `json:"totalWalletBalance"`
	TotalMarginBalance     fixedpoint.Value `json:"totalMarginBalance"`
	TotalAvailableBalance  fixedpoint.Value `json:"totalAvailableBalance"`
	TotalPerpUPL           fixedpoint.Value `json:"totalPerpUPL"`
	TotalInitialMargin     fixedpoint.Value `json:"totalInitialMargin"`
	TotalMaintenanceMargin fixedpoint.Value `json:"totalMaintenanceMargin"`
	// Account LTV: account total borrowed size / (account total equity + account total borrowed size).
	// In non-unified mode & unified (inverse) & unified (isolated_margin), the field will be returned as an empty string.
	AccountLTV fixedpoint.Value `json:"accountLTV"`
	Coins      []struct {
		Coin string `json:"coin"`
		// Equity of current coin
		Equity fixedpoint.Value `json:"equity"`
		// UsdValue of current coin. If this coin cannot be collateral, then it is 0
		UsdValue fixedpoint.Value `json:"usdValue"`
		// WalletBalance of current coin
		WalletBalance fixedpoint.Value `json:"walletBalance"`
		// Free available balance for Spot wallet. This is a unique field for Normal SPOT
		Free fixedpoint.Value
		// Locked balance for Spot wallet. This is a unique field for Normal SPOT
		Locked fixedpoint.Value
		// Available amount to withdraw of current coin
		AvailableToWithdraw fixedpoint.Value `json:"availableToWithdraw"`
		// Available amount to borrow of current coin
		AvailableToBorrow fixedpoint.Value `json:"availableToBorrow"`
		// Borrow amount of current coin
		BorrowAmount fixedpoint.Value `json:"borrowAmount"`
		// Accrued interest
		AccruedInterest fixedpoint.Value `json:"accruedInterest"`
		// Pre-occupied margin for order. For portfolio margin mode, it returns ""
		TotalOrderIM fixedpoint.Value `json:"totalOrderIM"`
		// Sum of initial margin of all positions + Pre-occupied liquidation fee. For portfolio margin mode, it returns ""
		TotalPositionIM fixedpoint.Value `json:"totalPositionIM"`
		// Sum of maintenance margin for all positions. For portfolio margin mode, it returns ""
		TotalPositionMM fixedpoint.Value `json:"totalPositionMM"`
		// Unrealised P&L
		UnrealisedPnl fixedpoint.Value `json:"unrealisedPnl"`
		// Cumulative Realised P&L
		CumRealisedPnl fixedpoint.Value `json:"cumRealisedPnl"`
		// Bonus. This is a unique field for UNIFIED account
		Bonus fixedpoint.Value `json:"bonus"`
		// Whether it can be used as a margin collateral currency (platform)
		// - When marginCollateral=false, then collateralSwitch is meaningless
		// -  This is a unique field for UNIFIED account
		CollateralSwitch bool `json:"collateralSwitch"`
		// Whether the collateral is turned on by user (user)
		// - When marginCollateral=true, then collateralSwitch is meaningful
		// - This is a unique field for UNIFIED account
		MarginCollateral bool `json:"marginCollateral"`
	} `json:"coin"`
}
