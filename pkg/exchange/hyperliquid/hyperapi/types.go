package hyperapi

import (
	"encoding/json"
	"fmt"
)

type SignatureResult struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

type ReqTypeInfo string

const (
	ReqMeta                      ReqTypeInfo = "meta"
	ReqSpotMeta                  ReqTypeInfo = "spotMeta"
	ReqSubmitOrder               ReqTypeInfo = "order"
	ReqCancelOrder               ReqTypeInfo = "cancel"
	ReqCandleSnapshot            ReqTypeInfo = "candleSnapshot"
	ReqFrontendOpenOrders        ReqTypeInfo = "frontendOpenOrders"
	ReqUserFills                 ReqTypeInfo = "userFills"
	ReqUserFillsByTime           ReqTypeInfo = "userFillsByTime"
	ReqHistoricalOrders          ReqTypeInfo = "historicalOrders"
	ReqSpotClearinghouseState    ReqTypeInfo = "spotClearinghouseState"
	ReqFuturesClearinghouseState ReqTypeInfo = "clearinghouseState"
	ReqSpotMetaAndAssetCtxs      ReqTypeInfo = "spotMetaAndAssetCtxs"
	ReqFuturesMetaAndAssetCtxs   ReqTypeInfo = "metaAndAssetCtxs"
)

type TimeInForce string

const (
	TimeInForceALO TimeInForce = "Alo"
	TimeInForceIOC TimeInForce = "Ioc"
	TimeInForceGTC TimeInForce = "Gtc"
)

type Grouping string

const (
	GroupingNA           Grouping = "na"
	GroupingNormalTpsl   Grouping = "normalTpsl"
	GroupingPositionTpls Grouping = "positionTpsl"
)

type Tpsl string // Advanced order type

const (
	TakeProfit Tpsl = "tp"
	StopLoss   Tpsl = "sl"
)

type APIResponse struct {
	Status   string `json:"status"`
	Response struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"response"`

	responseError string
}

func (a *APIResponse) Validate() error {
	if a.Status == "err" {
		if a.responseError != "" {
			return fmt.Errorf("%s", a.responseError)
		}
		return fmt.Errorf("%s", string(a.Response.Data))
	}
	return nil
}

// UnmarshalJSON handles API response where "response" can be either a string (when status is "err")
// or an object with "type" and "data" fields.
func (a *APIResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.Status = raw.Status
	if raw.Response == nil {
		return nil
	}

	if a.Status == "err" {
		var errMsg string
		if err := json.Unmarshal(raw.Response, &errMsg); err == nil {
			a.responseError = errMsg
			return nil
		}
		return fmt.Errorf("unknown error: %s", string(raw.Response))
	}
	return json.Unmarshal(raw.Response, &a.Response)
}
