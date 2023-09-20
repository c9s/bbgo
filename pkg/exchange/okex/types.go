package okex

import "github.com/c9s/bbgo/pkg/fixedpoint"

// login args
type WebsocketLogin struct {
	Key        string `json:"apiKey"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp"`
	Sign       string `json:"sign"`
}

// channel args
type WebsocketSubscription struct {
	Channel          string           `json:"channel"`
	InstrumentID     string           `json:"instId,omitempty"`
	InstrumentType   string           `json:"instType,omitempty"`
	InstrumentFamily string           `json:"instFamily,omitempty"`
	Side             string           `json:"side,omitempty"`
	TdMode           string           `json:"tdMode,omitempty"`
	OrderType        string           `json:"ordType,omitempty"`
	Quantity         fixedpoint.Value `json:"sz,omitempty"`
	Key              string           `json:"apiKey,omitempty"`
	Passphrase       string           `json:"passphrase,omitempty"`
	Timestamp        string           `json:"timestamp,omitempty"`
	Sign             string           `json:"sign,omitempty"`
}
