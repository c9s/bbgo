package hyperapi

import (
	"encoding/json"
)

type SignatureResult struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

type ReqTypeInfo string

const (
	ReqMeta               ReqTypeInfo = "meta"
	ReqSpotMeta           ReqTypeInfo = "spotMeta"
	ReqSubmitOrder        ReqTypeInfo = "order"
	ReqCancelOrder        ReqTypeInfo = "cancel"
	ReqCandleSnapshot     ReqTypeInfo = "candleSnapshot"
	ReqFrontendOpenOrders ReqTypeInfo = "frontendOpenOrders"
	ReqUserFills          ReqTypeInfo = "userFills"
	ReqUserFillsByTime    ReqTypeInfo = "userFillsByTime"
	ReqHistoricalOrders   ReqTypeInfo = "historicalOrders"
)

type TimeInForce string

const (
	TimeInForceALO TimeInForce = "Alo"
	TimeInForceIOC TimeInForce = "Ioc"
	TimeInForceGTC TimeInForce = "Gtc"
)

type APIResponse struct {
	Status   string `json:"status"`
	Response struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"response"`
}
