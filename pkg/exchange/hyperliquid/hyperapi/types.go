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
	ReqMeta        ReqTypeInfo = "meta"
	ReqSpotMeta    ReqTypeInfo = "spotMeta"
	ReqSubmitOrder ReqTypeInfo = "order"
	ReqCancelOrder ReqTypeInfo = "cancel"
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
