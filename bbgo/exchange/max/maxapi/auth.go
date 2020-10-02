package max

type AuthMessage struct {
	Action    string `json:"action"`
	APIKey    string `json:"apiKey"`
	Nonce     int64  `json:"nonce"`
	Signature string `json:"signature"`
	ID        string `json:"id"`
}

type AuthEvent struct {
	Event     string
	ID        string
	Timestamp int64
}
