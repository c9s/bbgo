package max

type AuthMessage struct {
	Action    string   `json:"action,omitempty"`
	APIKey    string   `json:"apiKey,omitempty"`
	Nonce     int64    `json:"nonce,omitempty"`
	Signature string   `json:"signature,omitempty"`
	ID        string   `json:"id,omitempty"`
	Filters   []string `json:"filters,omitempty"`
}

type AuthEvent struct {
	Event     string
	ID        string
	Timestamp int64
}
