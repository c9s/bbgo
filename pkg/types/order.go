package types

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/currency"
	"github.com/c9s/bbgo/pkg/util/templateutil"
)

func init() {
	// make sure we can cast Order to PlainText
	_ = PlainText(Order{})
	_ = PlainText(&Order{})
}

type CancelReplaceModeType string

var (
	StopOnFailure CancelReplaceModeType = "STOP_ON_FAILURE"
	AllowFailure  CancelReplaceModeType = "ALLOW_FAILURE"
)

type TimeInForce string

var (
	TimeInForceGTC TimeInForce = "GTC"
	TimeInForceIOC TimeInForce = "IOC"
	TimeInForceFOK TimeInForce = "FOK"
	TimeInForceGTT TimeInForce = "GTT" // for coinbase exchange api
)

// MarginOrderSideEffectType define side effect type for orders
type MarginOrderSideEffectType string

var (
	SideEffectTypeNoSideEffect    MarginOrderSideEffectType = "NO_SIDE_EFFECT"
	SideEffectTypeMarginBuy       MarginOrderSideEffectType = "MARGIN_BUY"
	SideEffectTypeAutoRepay       MarginOrderSideEffectType = "AUTO_REPAY"
	SideEffectTypeAutoBorrowRepay MarginOrderSideEffectType = "AUTO_BORROW_REPAY"
)

func (t *MarginOrderSideEffectType) UnmarshalJSON(data []byte) error {
	var s string
	var err = json.Unmarshal(data, &s)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal side effect type: %s", data)
	}

	switch strings.ToUpper(s) {

	case string(SideEffectTypeNoSideEffect), "":
		*t = SideEffectTypeNoSideEffect
		return nil

	case string(SideEffectTypeMarginBuy), "BORROW", "MARGINBUY":
		*t = SideEffectTypeMarginBuy
		return nil

	case string(SideEffectTypeAutoRepay), "REPAY", "AUTOREPAY":
		*t = SideEffectTypeAutoRepay
		return nil

	case string(SideEffectTypeAutoBorrowRepay), "BORROWREPAY", "AUTOBORROWREPAY":
		*t = SideEffectTypeAutoBorrowRepay
		return nil

	}

	return fmt.Errorf("invalid side effect type: %s", data)
}

// OrderType define order type
type OrderType string

const (
	OrderTypeLimit      OrderType = "LIMIT"
	OrderTypeLimitMaker OrderType = "LIMIT_MAKER"
	OrderTypeMarket     OrderType = "MARKET"
	OrderTypeStopLimit  OrderType = "STOP_LIMIT"
	OrderTypeStopMarket OrderType = "STOP_MARKET"

	OrderTypeTakeProfitMarket OrderType = "TAKE_PROFIT_MARKET"
)

/*
func (t *OrderType) Scan(v interface{}) error {
	switch d := v.(type) {
	case string:
		*t = OrderType(d)

	default:
		return errors.New("order type scan error, type unsupported")

	}
	return nil
}
*/

const NoClientOrderID = "0"

type OrderStatus string

const (
	// OrderStatusNew means the order is active on the orderbook without any filling.
	OrderStatusNew OrderStatus = "NEW"

	// OrderStatusFilled means the order is fully-filled, it's an end state.
	OrderStatusFilled OrderStatus = "FILLED"

	// OrderStatusPartiallyFilled means the order is partially-filled, it's an end state, the order might be canceled in the end.
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"

	// OrderStatusCanceled means the order is canceled without partially filled or filled.
	OrderStatusCanceled OrderStatus = "CANCELED"

	// OrderStatusRejected means the order is not placed successfully, it's rejected by the api
	OrderStatusRejected OrderStatus = "REJECTED"

	// OrderStatusExpired means the order is expired, it's an end state.
	OrderStatusExpired OrderStatus = "EXPIRED"
)

func (o OrderStatus) Closed() bool {
	return o == OrderStatusFilled ||
		o == OrderStatusCanceled ||
		o == OrderStatusRejected ||
		o == OrderStatusExpired
}

type SubmitOrder struct {
	ClientOrderID string `json:"clientOrderID,omitempty" db:"client_order_id"`

	// QuoteID is for OTC exchange
	QuoteID string `json:"quoteID,omitempty" db:"quote_id"`

	Symbol string    `json:"symbol" db:"symbol"`
	Side   SideType  `json:"side" db:"side"`
	Type   OrderType `json:"orderType" db:"order_type"`

	Quantity fixedpoint.Value `json:"quantity" db:"quantity"`
	Price    fixedpoint.Value `json:"price" db:"price"`

	// AveragePrice is only used in back-test currently
	AveragePrice fixedpoint.Value `json:"averagePrice,omitempty"`

	StopPrice fixedpoint.Value `json:"stopPrice,omitempty" db:"stop_price"`

	Market Market `json:"-" db:"-"`

	TimeInForce TimeInForce `json:"timeInForce,omitempty" db:"time_in_force"` // GTC, IOC, FOK

	GroupID uint32 `json:"groupID,omitempty"`

	MarginSideEffect MarginOrderSideEffectType `json:"marginSideEffect,omitempty"` // AUTO_BORROW_REPAY = borrowrepay, AUTO_REPAY = repay, MARGIN_BUY = borrow, defaults to  NO_SIDE_EFFECT

	ReduceOnly bool `json:"reduceOnly,omitempty" db:"reduce_only"`

	// this is mostly designed for binance: true, false；Close-All，used with STOP_MARKET or TAKE_PROFIT_MARKET.
	ClosePosition bool `json:"closePosition,omitempty" db:"close_position"`

	Tag string `json:"tag,omitempty" db:"-"`
}

func (o *SubmitOrder) LogFields() logrus.Fields {
	fields := logrus.Fields{
		"symbol": o.Symbol,
	}

	if len(o.ClientOrderID) > 0 {
		fields["client_order_id"] = o.ClientOrderID
	}

	return fields
}

func (o *SubmitOrder) AsQuery() OrderQuery {
	return OrderQuery{
		Symbol:        o.Symbol,
		ClientOrderID: o.ClientOrderID,
	}
}

func (o *SubmitOrder) In() (fixedpoint.Value, string) {
	switch o.Side {
	case SideTypeBuy:
		if o.AveragePrice.IsZero() {
			return o.Quantity.Mul(o.Price), o.Market.QuoteCurrency
		} else {
			return o.Quantity.Mul(o.AveragePrice), o.Market.QuoteCurrency
		}

	case SideTypeSell:
		return o.Quantity, o.Market.BaseCurrency

	}

	return fixedpoint.Zero, ""
}

func (o *SubmitOrder) Out() (fixedpoint.Value, string) {
	switch o.Side {
	case SideTypeBuy:
		return o.Quantity, o.Market.BaseCurrency

	case SideTypeSell:
		if o.AveragePrice.IsZero() {
			return o.Quantity.Mul(o.Price), o.Market.QuoteCurrency
		} else {
			return o.Quantity.Mul(o.AveragePrice), o.Market.QuoteCurrency
		}
	}

	return fixedpoint.Zero, ""
}

func (o *SubmitOrder) String() string {
	switch o.Type {
	case OrderTypeMarket:
		return fmt.Sprintf("SubmitOrder %s %s %s %s", o.Symbol, o.Type, o.Side, o.Quantity.String())
	}

	return fmt.Sprintf("SubmitOrder %s %s %s %s @ %s", o.Symbol, o.Type, o.Side, o.Quantity.String(), o.Price.String())
}

func (o *SubmitOrder) PlainText() string {
	switch o.Type {
	case OrderTypeMarket:
		return fmt.Sprintf("SubmitOrder %s %s %s %s", o.Symbol, o.Type, o.Side, o.Quantity.String())
	}

	return fmt.Sprintf("SubmitOrder %s %s %s %s @ %s", o.Symbol, o.Type, o.Side, o.Quantity.String(), o.Price.String())
}

func (o *SubmitOrder) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Price", Value: o.Price.String(), Short: true},
		{Title: "Quantity", Value: o.Quantity.String(), Short: true},
	}

	if o.Price.Sign() > 0 && o.Quantity.Sign() > 0 && len(o.Market.QuoteCurrency) > 0 {
		if currency.IsFiatCurrency(o.Market.QuoteCurrency) {
			fields = append(fields, slack.AttachmentField{
				Title: "Amount",
				Value: currency.USD.FormatMoney(o.Price.Mul(o.Quantity)),
				Short: true,
			})
		} else {
			fields = append(fields, slack.AttachmentField{
				Title: "Amount",
				Value: fmt.Sprintf("%s %s", o.Price.Mul(o.Quantity).String(), o.Market.QuoteCurrency),
				Short: true,
			})
		}
	}

	if len(o.ClientOrderID) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "ClientOrderID", Value: o.ClientOrderID, Short: true})
	}

	if len(o.MarginSideEffect) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "MarginSideEffect", Value: string(o.MarginSideEffect), Short: true})
	}

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields: fields,
	}
}

type OrderQuery struct {
	Symbol        string
	OrderID       string
	ClientOrderID string
	OrderUUID     string
}

type Order struct {
	SubmitOrder

	Exchange ExchangeName `json:"exchange" db:"exchange"`

	// GID is used for relational database storage, it's an incremental ID
	GID     uint64 `json:"gid,omitempty" db:"gid"`
	OrderID uint64 `json:"orderID" db:"order_id"` // order id
	UUID    string `json:"uuid,omitempty" db:"uuid"`

	Status OrderStatus `json:"status" db:"status"`

	// OriginalStatus stores the original order status from the specific exchange
	OriginalStatus string `json:"originalStatus,omitempty" db:"-"`

	// ExecutedQuantity is how much quantity has been executed
	ExecutedQuantity fixedpoint.Value `json:"executedQuantity" db:"executed_quantity"`

	// IsWorking means if the order is still on the order book (active order)
	IsWorking bool `json:"isWorking" db:"is_working"`

	// CreationTime is the time when this order is created
	CreationTime Time `json:"creationTime" db:"created_at"`

	// UpdateTime is the latest time when this order gets updated
	UpdateTime Time `json:"updateTime" db:"updated_at"`

	IsFutures  bool `json:"isFutures,omitempty" db:"is_futures"`
	IsMargin   bool `json:"isMargin,omitempty" db:"is_margin"`
	IsIsolated bool `json:"isIsolated,omitempty" db:"is_isolated"`
}

func (o *Order) LogFields() logrus.Fields {
	fields := o.SubmitOrder.LogFields()
	fields["exchange"] = o.Exchange.String()
	fields["order_id"] = o.OrderID
	fields["status"] = o.Status

	if len(o.UUID) > 0 {
		fields["uuid"] = o.UUID
	}

	if len(o.ClientOrderID) > 0 {
		fields["client_order_id"] = o.ClientOrderID
	}

	return fields
}

func (o *Order) Update(update Order) {
	// if the update time is older than the current order, ignore it
	if update.UpdateTime.Before(o.UpdateTime.Time()) {
		return
	}
	o.UpdateTime = update.UpdateTime
	if len(update.Status) > 0 {
		o.Status = update.Status
	}
	if len(update.OriginalStatus) > 0 {
		o.OriginalStatus = update.OriginalStatus
	}
	// executed quantity should be increasing
	if update.ExecutedQuantity.Compare(o.ExecutedQuantity) > 0 {
		o.ExecutedQuantity = update.ExecutedQuantity
	}
	// update quantity and price only if the update value is greater than 0
	if update.Quantity.Sign() > 0 {
		o.Quantity = update.Quantity
	}
	if update.Price.Sign() > 0 {
		o.Price = update.Price
	}
	o.IsWorking = update.IsWorking
}

func (o *Order) GetRemainingQuantity() fixedpoint.Value {
	return o.Quantity.Sub(o.ExecutedQuantity)
}

func (o Order) AsQuery() OrderQuery {
	return OrderQuery{
		Symbol:    o.Symbol,
		OrderID:   strconv.FormatUint(o.OrderID, 10),
		OrderUUID: o.UUID,
	}
}

func (o Order) CsvHeader() []string {
	return []string{
		"order_id",
		"symbol",
		"side",
		"order_type",
		"status",
		"price",
		"quantity",
		"creation_time",
		"update_time",
		"tag",
	}
}

func (o Order) CsvRecords() [][]string {
	return [][]string{
		{
			strconv.FormatUint(o.OrderID, 10),
			o.Symbol,
			string(o.Side),
			string(o.Type),
			string(o.Status),
			o.Price.String(),
			o.Quantity.String(),
			o.CreationTime.Time().Local().Format(time.RFC1123),
			o.UpdateTime.Time().Local().Format(time.RFC1123),
			o.Tag,
		},
	}
}

func (o *Order) ObjectID() string {
	return "order-" + o.Exchange.String() + "-" + o.Symbol + "-" + strconv.FormatUint(o.OrderID, 10)
}

// Backup backs up the current order quantity to a SubmitOrder object
// so that we can post the order later when we want to restore the orders.
func (o Order) Backup() SubmitOrder {
	so := o.SubmitOrder
	so.Quantity = o.Quantity.Sub(o.ExecutedQuantity)

	// ClientOrderID can not be reused
	so.ClientOrderID = ""
	return so
}

func (o Order) String() string {
	var orderID string
	if o.UUID != "" {
		orderID = fmt.Sprintf("UUID %s (%d)", o.UUID, o.OrderID)
	} else {
		orderID = strconv.FormatUint(o.OrderID, 10)
	}

	desc := fmt.Sprintf("ORDER %s | %s | %s | %s %-4s |",
		o.Exchange.String(),
		orderID,
		o.Symbol,
		o.Type,
		o.Side,
	)

	switch o.Type {
	case OrderTypeStopLimit, OrderTypeStopMarket, OrderTypeTakeProfitMarket:
		desc += " stop@ " + o.StopPrice.String() + " ->"
	}

	desc += fmt.Sprintf(" %s/%s @ %s", o.ExecutedQuantity.String(),
		o.Quantity.String(),
		o.Price.String())

	desc += " | " + string(o.Status) + " | "

	desc += time.Time(o.CreationTime).UTC().Format(time.StampMilli)

	if time.Time(o.UpdateTime).IsZero() {
		desc += " -> 0"
	} else {
		desc += " -> " + time.Time(o.UpdateTime).UTC().Format(time.StampMilli)
	}

	return desc
}

// PlainText is used for telegram-styled messages
func (o Order) PlainText() string {
	return fmt.Sprintf("Order %s %s %s %s @ %s %s/%s -> %s",
		o.Exchange.String(),
		o.Symbol,
		o.Type,
		o.Side,
		o.Price.String(),
		o.ExecutedQuantity.String(),
		o.Quantity.String(),
		o.Status)
}

func (o Order) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Price", Value: o.Price.String(), Short: true},
		{
			Title: "Executed Quantity",
			Value: o.ExecutedQuantity.String() + "/" + o.Quantity.String(),
			Short: true,
		},
	}

	fields = append(fields, slack.AttachmentField{
		Title: "ID",
		Value: strconv.FormatUint(o.OrderID, 10),
		Short: true,
	})

	orderStatusIcon := ""

	switch o.Status {
	case OrderStatusNew:
		orderStatusIcon = ":new:"
	case OrderStatusCanceled:
		orderStatusIcon = ":eject:"
	case OrderStatusPartiallyFilled:
		orderStatusIcon = ":arrow_forward:"
	case OrderStatusFilled:
		orderStatusIcon = ":white_check_mark:"

	}

	fields = append(fields, slack.AttachmentField{
		Title: "Status",
		Value: string(o.Status) + " " + orderStatusIcon,
		Short: true,
	})

	footerIcon := ExchangeFooterIcon(o.Exchange)

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields:     fields,
		FooterIcon: footerIcon,
		Footer:     strings.ToLower(o.Exchange.String()) + templateutil.Render(" creation time {{ . }}", o.CreationTime.Time().Format(time.StampMilli)),
	}
}

func OrdersFilter(in []Order, f func(o Order) bool) (out []Order) {
	for _, o := range in {
		if f(o) {
			out = append(out, o)
		}
	}
	return out
}

func OrdersActive(in []Order) []Order {
	return OrdersFilter(in, IsActiveOrder)
}

func OrdersFilled(in []Order) (out []Order) {
	return OrdersFilter(in, func(o Order) bool {
		return o.Status == OrderStatusFilled
	})
}

func OrdersAll(orders []Order, f func(o Order) bool) bool {
	for _, o := range orders {
		if !f(o) {
			return false
		}
	}
	return true
}

func OrdersAny(orders []Order, f func(o Order) bool) bool {
	return slices.ContainsFunc(orders, f)
}

func IsActiveOrder(o Order) bool {
	return o.Status == OrderStatusNew || o.Status == OrderStatusPartiallyFilled
}
