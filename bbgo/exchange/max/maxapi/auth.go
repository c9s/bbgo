package max

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

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

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}

