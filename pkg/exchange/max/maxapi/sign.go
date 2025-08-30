package maxapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func SignPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}
