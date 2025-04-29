package binanceapi

import (
	"crypto/ed25519"
	"encoding/base64"
)

// GenerateSignatureEd25519 generates a signature for the given string with the provided private key.
func GenerateSignatureEd25519(content string, privateKey ed25519.PrivateKey) string {
	signatureBytes := ed25519.Sign(privateKey, []byte(content))
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
	return signature
}
