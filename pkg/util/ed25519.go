package util

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func ParseEd25519PrivateKey(pemString string) (ed25519.PrivateKey, error) {
	if len(pemString) == 0 {
		return nil, errors.New("empty PEM string")
	}
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the private key")
	}

	// Parse the private key
	pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := pkcs8Key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ED25519 private key: %T given", pkcs8Key)
	}
	return privateKey, nil
}
