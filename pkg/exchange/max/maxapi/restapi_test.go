package maxapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNonce(t *testing.T) {
	client := RestClient{
		apiKeyNonce: map[string]int64{},
	}

	var nonces []int64
	for i := 0; i < 1200; i++ {
		nonce := client.GetNonce("abc")
		t.Logf("nonce: %v", nonce)
		if i > 0 {
			assert.Greater(t, nonce, nonces[i-1])
		}
		nonces = append(nonces, nonce)
	}
}
