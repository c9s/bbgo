package backpackapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// newTestKeyPair returns a deterministic ed25519 key pair encoded the way the Backpack console
// issues credentials: the api key is the base64 verifying key, the api secret is the base64
// 32-byte seed.
func newTestKeyPair(seedByte byte) (key, secret string) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = seedByte
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	return base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		base64.StdEncoding.EncodeToString(seed)
}

func TestBuildSigningString(t *testing.T) {
	t.Run("documented orderCancel example", func(t *testing.T) {
		// from the docs: cancelling an order with the body {"orderId": 28, "symbol": "BTC_USDT"}
		params := signingParams(nil, map[string]interface{}{
			"orderId": "28",
			"symbol":  "BTC_USDT",
		})

		assert.Equal(t,
			"instruction=orderCancel&orderId=28&symbol=BTC_USDT&timestamp=1614550000000&window=5000",
			buildSigningString(InstructionOrderCancel, params, 1614550000000, 5000))
	})

	t.Run("no parameters", func(t *testing.T) {
		assert.Equal(t,
			"instruction=balanceQuery&timestamp=1614550000000&window=5000",
			buildSigningString(InstructionBalanceQuery, signingParams(nil, nil), 1614550000000, 5000))
	})

	t.Run("query parameters are sorted alphabetically", func(t *testing.T) {
		query := url.Values{}
		query.Set("symbol", "SOL_USDC")
		query.Set("marketType", "SPOT")
		query.Set("limit", "100")

		assert.Equal(t,
			"instruction=orderQueryAll&limit=100&marketType=SPOT&symbol=SOL_USDC&timestamp=1&window=5000",
			buildSigningString(InstructionOrderQueryAll, signingParams(query, nil), 1, 5000))
	})

	t.Run("body parameters take precedence over the query", func(t *testing.T) {
		query := url.Values{}
		query.Set("ignored", "1")

		params := signingParams(query, map[string]interface{}{"symbol": "SOL_USDC"})
		assert.Equal(t, []string{"symbol=SOL_USDC"}, params)
	})

	t.Run("booleans and numbers are formatted without an exponent", func(t *testing.T) {
		params := signingParams(nil, map[string]interface{}{
			"postOnly": true,
			"clientId": float64(4294967295), // what a JSON round trip produces
			"quantity": "0.5",
		})

		assert.Equal(t,
			[]string{"clientId=4294967295", "postOnly=true", "quantity=0.5"},
			params)
	})
}

func TestBuildBatchSigningString(t *testing.T) {
	// the batch example from the docs
	orders := []OrderExecutePayload{
		{
			Symbol:    "SOL_USDC_PERP",
			Side:      SideBid,
			OrderType: OrderTypeLimit,
			Price:     "141",
			Quantity:  "12",
		},
		{
			Symbol:    "SOL_USDC_PERP",
			Side:      SideBid,
			OrderType: OrderTypeLimit,
			Price:     "140",
			Quantity:  "11",
		},
	}

	got, err := buildBatchSigningString(orders, 1750793021519, 5000)
	if assert.NoError(t, err) {
		assert.Equal(t,
			"instruction=orderExecute&orderType=Limit&price=141&quantity=12&side=Bid&symbol=SOL_USDC_PERP"+
				"&instruction=orderExecute&orderType=Limit&price=140&quantity=11&side=Bid&symbol=SOL_USDC_PERP"+
				"&timestamp=1750793021519&window=5000",
			got)
	}
}

func TestRestClient_Auth(t *testing.T) {
	key, secret := newTestKeyPair(0x01)

	t.Run("valid credentials", func(t *testing.T) {
		client := NewClient()
		if assert.NoError(t, client.Auth(key, secret)) {
			assert.Equal(t, key, client.key)
			assert.NotNil(t, client.privateKey)
		}
	})

	t.Run("signature verifies against the api key", func(t *testing.T) {
		client := NewClient()
		assert.NoError(t, client.Auth(key, secret))

		message := buildSigningString(InstructionBalanceQuery, nil, 1614550000000, 5000)

		signature, err := base64.StdEncoding.DecodeString(client.sign(message))
		if assert.NoError(t, err) {
			publicKey, err := base64.StdEncoding.DecodeString(key)
			if assert.NoError(t, err) {
				assert.True(t, ed25519.Verify(publicKey, []byte(message), signature),
					"the signature should verify against the api key")
			}
		}
	})

	t.Run("rejects an empty key or secret", func(t *testing.T) {
		client := NewClient()
		assert.ErrorIs(t, client.Auth("", secret), errNoApiKey)
		assert.ErrorIs(t, client.Auth(key, ""), errNoApiSecret)
	})

	t.Run("rejects a secret that is not base64", func(t *testing.T) {
		client := NewClient()
		assert.Error(t, client.Auth(key, "not base64!!"))
	})

	t.Run("rejects a secret of the wrong length", func(t *testing.T) {
		client := NewClient()
		assert.ErrorContains(t, client.Auth(key, base64.StdEncoding.EncodeToString([]byte("short"))),
			"ed25519 seed")
	})

	t.Run("rejects a key that does not belong to the secret", func(t *testing.T) {
		otherKey, _ := newTestKeyPair(0x02)

		client := NewClient()
		assert.ErrorContains(t, client.Auth(otherKey, secret), "does not match")
	})
}

func TestRestClient_authenticatedRequestRequiresAnInstruction(t *testing.T) {
	key, secret := newTestKeyPair(0x03)

	client := NewClient()
	assert.NoError(t, client.Auth(key, secret))

	// calling the client directly, without binding an instruction, must not silently sign an
	// unusable request
	_, err := client.NewAuthenticatedRequest(t.Context(), "GET", "/api/v1/capital", nil, nil)
	assert.ErrorContains(t, err, "instruction")
}

func TestInstructionClient_setsAuthHeaders(t *testing.T) {
	key, secret := newTestKeyPair(0x04)

	client := NewClient()
	assert.NoError(t, client.Auth(key, secret))

	authed := client.withInstruction(InstructionBalanceQuery)
	req, err := authed.NewAuthenticatedRequest(t.Context(), "GET", "/api/v1/capital", nil, nil)
	if assert.NoError(t, err) {
		assert.Equal(t, key, req.Header.Get("X-API-Key"))
		assert.NotEmpty(t, req.Header.Get("X-Signature"))
		assert.NotEmpty(t, req.Header.Get("X-Timestamp"))
		assert.Equal(t, "5000", req.Header.Get("X-Window"))
	}
}

func TestRestClient_SetWindow(t *testing.T) {
	client := NewClient()
	assert.Equal(t, defaultWindow, client.getWindow())

	client.SetWindow(10000)
	assert.Equal(t, uint64(10000), client.getWindow())

	// the api rejects anything above 60000
	client.SetWindow(120000)
	assert.Equal(t, maxWindow, client.getWindow())
}
