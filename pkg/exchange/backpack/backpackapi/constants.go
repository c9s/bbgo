package backpackapi

// Instruction is the endpoint-specific instruction name that is prefixed to the signing
// string of an authenticated request.
type Instruction string

const (
	InstructionBalanceQuery         Instruction = "balanceQuery"
	InstructionCollateralQuery      Instruction = "collateralQuery"
	InstructionPositionQuery        Instruction = "positionQuery"
	InstructionOrderExecute         Instruction = "orderExecute"
	InstructionOrderQuery           Instruction = "orderQuery"
	InstructionOrderQueryAll        Instruction = "orderQueryAll"
	InstructionOrderCancel          Instruction = "orderCancel"
	InstructionOrderCancelAll       Instruction = "orderCancelAll"
	InstructionOrderHistoryQueryAll Instruction = "orderHistoryQueryAll"
	InstructionFillHistoryQueryAll  Instruction = "fillHistoryQueryAll"
)

// Side is the order side. Note that Backpack uses Bid/Ask rather than BUY/SELL.
type Side string

const (
	// SideBid is the buy side.
	SideBid Side = "Bid"
	// SideAsk is the sell side.
	SideAsk Side = "Ask"
)

type OrderType string

const (
	OrderTypeMarket OrderType = "Market"
	OrderTypeLimit  OrderType = "Limit"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC"
	TimeInForceIOC TimeInForce = "IOC"
	TimeInForceFOK TimeInForce = "FOK"
)

type SelfTradePrevention string

const (
	SelfTradePreventionRejectTaker SelfTradePrevention = "RejectTaker"
	SelfTradePreventionRejectMaker SelfTradePrevention = "RejectMaker"
	SelfTradePreventionRejectBoth  SelfTradePrevention = "RejectBoth"
)

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "New"
	OrderStatusPartiallyFilled OrderStatus = "PartiallyFilled"
	OrderStatusFilled          OrderStatus = "Filled"
	OrderStatusCancelled       OrderStatus = "Cancelled"
	OrderStatusExpired         OrderStatus = "Expired"
	OrderStatusTriggerPending  OrderStatus = "TriggerPending"
	OrderStatusTriggerFailed   OrderStatus = "TriggerFailed"
)

func (s OrderStatus) IsWorking() bool {
	return s == OrderStatusNew || s == OrderStatusPartiallyFilled || s == OrderStatusTriggerPending
}

type MarketType string

const (
	MarketTypeSpot       MarketType = "SPOT"
	MarketTypePerp       MarketType = "PERP"
	MarketTypeIPerp      MarketType = "IPERP"
	MarketTypeDated      MarketType = "DATED"
	MarketTypePrediction MarketType = "PREDICTION"
	MarketTypeRfq        MarketType = "RFQ"
)

type OrderBookState string

const (
	OrderBookStateOpen       OrderBookState = "Open"
	OrderBookStateClosed     OrderBookState = "Closed"
	OrderBookStateCancelOnly OrderBookState = "CancelOnly"
	OrderBookStateLimitOnly  OrderBookState = "LimitOnly"
	OrderBookStatePostOnly   OrderBookState = "PostOnly"
)

type KlineInterval string

const (
	KlineInterval1s     KlineInterval = "1s"
	KlineInterval1m     KlineInterval = "1m"
	KlineInterval3m     KlineInterval = "3m"
	KlineInterval5m     KlineInterval = "5m"
	KlineInterval15m    KlineInterval = "15m"
	KlineInterval30m    KlineInterval = "30m"
	KlineInterval1h     KlineInterval = "1h"
	KlineInterval2h     KlineInterval = "2h"
	KlineInterval4h     KlineInterval = "4h"
	KlineInterval6h     KlineInterval = "6h"
	KlineInterval8h     KlineInterval = "8h"
	KlineInterval12h    KlineInterval = "12h"
	KlineInterval1d     KlineInterval = "1d"
	KlineInterval3d     KlineInterval = "3d"
	KlineInterval1w     KlineInterval = "1w"
	KlineInterval1month KlineInterval = "1month"
)

type KlinePriceType string

const (
	KlinePriceTypeLast  KlinePriceType = "Last"
	KlinePriceTypeIndex KlinePriceType = "Index"
	KlinePriceTypeMark  KlinePriceType = "Mark"
)

type KlineSource string

const (
	KlineSourceVenue    KlineSource = "Venue"
	KlineSourceExternal KlineSource = "External"
)

type TickerInterval string

const (
	TickerInterval1d TickerInterval = "1d"
	TickerInterval1w TickerInterval = "1w"
)

// DepthLimit is the number of order book levels to return. The API expects a string.
type DepthLimit string

const (
	DepthLimit5    DepthLimit = "5"
	DepthLimit10   DepthLimit = "10"
	DepthLimit20   DepthLimit = "20"
	DepthLimit50   DepthLimit = "50"
	DepthLimit100  DepthLimit = "100"
	DepthLimit500  DepthLimit = "500"
	DepthLimit1000 DepthLimit = "1000"
)

// CancelOrderType selects which kind of orders DELETE /api/v1/orders cancels.
type CancelOrderType string

const (
	CancelOrderTypeRestingLimitOrder CancelOrderType = "RestingLimitOrder"
	CancelOrderTypeConditionalOrder  CancelOrderType = "ConditionalOrder"
)

type SortDirection string

const (
	SortDirectionAsc  SortDirection = "Asc"
	SortDirectionDesc SortDirection = "Desc"
)

type FillType string

const (
	FillTypeUser                                   FillType = "User"
	FillTypeBookLiquidation                        FillType = "BookLiquidation"
	FillTypeAdl                                    FillType = "Adl"
	FillTypeBackstop                               FillType = "Backstop"
	FillTypeLiquidation                            FillType = "Liquidation"
	FillTypeAllLiquidation                         FillType = "AllLiquidation"
	FillTypeCollateralConversion                   FillType = "CollateralConversion"
	FillTypeCollateralConversionAndSpotLiquidation FillType = "CollateralConversionAndSpotLiquidation"
)

type SystemOrderType string

const (
	SystemOrderTypeCollateralConversion        SystemOrderType = "CollateralConversion"
	SystemOrderTypeFutureExpiry                SystemOrderType = "FutureExpiry"
	SystemOrderTypeLiquidatePositionOnAdl      SystemOrderType = "LiquidatePositionOnAdl"
	SystemOrderTypeLiquidatePositionOnBook     SystemOrderType = "LiquidatePositionOnBook"
	SystemOrderTypeLiquidatePositionOnBackstop SystemOrderType = "LiquidatePositionOnBackstop"
	SystemOrderTypeOrderBookClosed             SystemOrderType = "OrderBookClosed"
)

// TriggerBy selects the price that a conditional order triggers on.
type TriggerBy string

const (
	TriggerByMarkPrice  TriggerBy = "MarkPrice"
	TriggerByLastPrice  TriggerBy = "LastPrice"
	TriggerByIndexPrice TriggerBy = "IndexPrice"
)

type SlippageToleranceType string

const (
	SlippageToleranceTypeTickSize SlippageToleranceType = "TickSize"
	SlippageToleranceTypePercent  SlippageToleranceType = "Percent"
)

// SystemStatus is the exchange status returned by GET /api/v1/status.
type SystemStatus string

const (
	SystemStatusOk          SystemStatus = "Ok"
	SystemStatusMaintenance SystemStatus = "Maintenance"
)

// ApiErrorCode is the error code carried by an error response.
type ApiErrorCode string

const (
	ApiErrorCodeAccountDeactivated       ApiErrorCode = "ACCOUNT_DEACTIVATED"
	ApiErrorCodeAccountLiquidating       ApiErrorCode = "ACCOUNT_LIQUIDATING"
	ApiErrorCodeBorrowLimit              ApiErrorCode = "BORROW_LIMIT"
	ApiErrorCodeBorrowRequiresLendRedeem ApiErrorCode = "BORROW_REQUIRES_LEND_REDEEM"
	ApiErrorCodeForbidden                ApiErrorCode = "FORBIDDEN"
	ApiErrorCodeInsufficientFunds        ApiErrorCode = "INSUFFICIENT_FUNDS"
	ApiErrorCodeInsufficientMargin       ApiErrorCode = "INSUFFICIENT_MARGIN"
	ApiErrorCodeInsufficientSupply       ApiErrorCode = "INSUFFICIENT_SUPPLY"
	ApiErrorCodeInvalidAsset             ApiErrorCode = "INVALID_ASSET"
	ApiErrorCodeInvalidClientRequest     ApiErrorCode = "INVALID_CLIENT_REQUEST"
	ApiErrorCodeInvalidMarket            ApiErrorCode = "INVALID_MARKET"
	ApiErrorCodeInvalidOrder             ApiErrorCode = "INVALID_ORDER"
	ApiErrorCodeInvalidPrice             ApiErrorCode = "INVALID_PRICE"
	ApiErrorCodeInvalidPositionId        ApiErrorCode = "INVALID_POSITION_ID"
	ApiErrorCodeInvalidQuantity          ApiErrorCode = "INVALID_QUANTITY"
	ApiErrorCodeInvalidRange             ApiErrorCode = "INVALID_RANGE"
	ApiErrorCodeInvalidSignature         ApiErrorCode = "INVALID_SIGNATURE"
	ApiErrorCodeInvalidSource            ApiErrorCode = "INVALID_SOURCE"
	ApiErrorCodeInvalidSymbol            ApiErrorCode = "INVALID_SYMBOL"
	ApiErrorCodeInvalidTwoFactorCode     ApiErrorCode = "INVALID_TWO_FACTOR_CODE"
	ApiErrorCodeLendLimit                ApiErrorCode = "LEND_LIMIT"
	ApiErrorCodeLendRequiresBorrowRepay  ApiErrorCode = "LEND_REQUIRES_BORROW_REPAY"
	ApiErrorCodeMaintenance              ApiErrorCode = "MAINTENANCE"
	ApiErrorCodeMaxLeverageReached       ApiErrorCode = "MAX_LEVERAGE_REACHED"
	ApiErrorCodeOrderLimit               ApiErrorCode = "ORDER_LIMIT"
	ApiErrorCodePositionLimit            ApiErrorCode = "POSITION_LIMIT"
	ApiErrorCodePreconditionFailed       ApiErrorCode = "PRECONDITION_FAILED"
	ApiErrorCodeResourceNotFound         ApiErrorCode = "RESOURCE_NOT_FOUND"
	ApiErrorCodeServerError              ApiErrorCode = "SERVER_ERROR"
	ApiErrorCodeTimeout                  ApiErrorCode = "TIMEOUT"
	ApiErrorCodeTooEarly                 ApiErrorCode = "TOO_EARLY"
	ApiErrorCodeTooManyRequests          ApiErrorCode = "TOO_MANY_REQUESTS"
	ApiErrorCodeTradingPaused            ApiErrorCode = "TRADING_PAUSED"
	ApiErrorCodeUnauthorized             ApiErrorCode = "UNAUTHORIZED"
)
