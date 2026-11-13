package binanceapi

import "github.com/c9s/requestgen"

type FuturesExchangeInfo struct {
	Timezone    string `json:"timezone"`
	ServerTime  int64  `json:"serverTime"`
	FuturesType string `json:"futuresType"`
	RateLimits  []struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		IntervalNum   int    `json:"intervalNum"`
		Limit         int    `json:"limit"`
	} `json:"rateLimits"`
	ExchangeFilters []any `json:"exchangeFilters"`
	Assets          []struct {
		Asset             string `json:"asset"`
		MarginAvailable   bool   `json:"marginAvailable"`
		AutoAssetExchange string `json:"autoAssetExchange"`
	} `json:"assets"`
	Symbols []struct {
		Symbol                string   `json:"symbol"`
		Pair                  string   `json:"pair"`
		ContractType          string   `json:"contractType"`
		DeliveryDate          int64    `json:"deliveryDate"`
		OnboardDate           int64    `json:"onboardDate"`
		Status                string   `json:"status"`
		MaintMarginPercent    string   `json:"maintMarginPercent"`
		RequiredMarginPercent string   `json:"requiredMarginPercent"`
		BaseAsset             string   `json:"baseAsset"`
		QuoteAsset            string   `json:"quoteAsset"`
		MarginAsset           string   `json:"marginAsset"`
		PricePrecision        int      `json:"pricePrecision"`
		QuantityPrecision     int      `json:"quantityPrecision"`
		BaseAssetPrecision    int      `json:"baseAssetPrecision"`
		QuotePrecision        int      `json:"quotePrecision"`
		UnderlyingType        string   `json:"underlyingType"`
		UnderlyingSubType     []string `json:"underlyingSubType"`
		TriggerProtect        string   `json:"triggerProtect"`
		LiquidationFee        string   `json:"liquidationFee"`
		MarketTakeBound       string   `json:"marketTakeBound"`
		MaxMoveOrderLimit     int      `json:"maxMoveOrderLimit"`
		Filters               []struct {
			TickSize            string `json:"tickSize,omitempty"`
			MinPrice            string `json:"minPrice,omitempty"`
			FilterType          string `json:"filterType"`
			MaxPrice            string `json:"maxPrice,omitempty"`
			StepSize            string `json:"stepSize,omitempty"`
			MinQty              string `json:"minQty,omitempty"`
			MaxQty              string `json:"maxQty,omitempty"`
			Limit               int    `json:"limit,omitempty"`
			Notional            string `json:"notional,omitempty"`
			MultiplierDown      string `json:"multiplierDown,omitempty"`
			MultiplierUp        string `json:"multiplierUp,omitempty"`
			MultiplierDecimal   string `json:"multiplierDecimal,omitempty"`
			PositionControlSide string `json:"positionControlSide,omitempty"`
		} `json:"filters"`
		OrderTypes     []string `json:"orderTypes"`
		TimeInForce    []string `json:"timeInForce"`
		PermissionSets []string `json:"permissionSets"`
	} `json:"symbols"`
}

//go:generate requestgen -method GET -url "/fapi/v1/exchangeInfo" -type FuturesExchangeInfoRequest -responseType FuturesExchangeInfo
type FuturesExchangeInfoRequest struct {
	client requestgen.APIClient
}

func (c *FuturesRestClient) NewFuturesExchangeInfoRequest() *FuturesExchangeInfoRequest {
	return &FuturesExchangeInfoRequest{
		client: c,
	}
}
