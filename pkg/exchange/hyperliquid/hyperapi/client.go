package hyperapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/bbgo/pkg/nonce"
	"github.com/c9s/requestgen"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	ProductionURL      = "https://api.hyperliquid.xyz"
	TestNetURL         = "https://api.hyperliquid-testnet.xyz"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	TestNet             = false
)

type Client struct {
	requestgen.BaseAPIClient

	apiSecret    string
	vaultAddress string
	privateKey   *ecdsa.PrivateKey

	nonce *nonce.MillisecondNonce
}

func NewClient() *Client {
	u, err := url.Parse(getAPIEndpoint())
	if err != nil {
		panic(err)
	}

	return &Client{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: u,
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
		nonce: nonce.NewMillisecondNonce(time.Now()),
	}
}

func (c *Client) Auth(secret string) {
	c.apiSecret = secret

	privateKey, err := crypto.HexToECDSA(c.apiSecret)
	if err != nil {
		panic(err)
	}
	c.privateKey = privateKey
}

func (c *Client) SetVaultAddress(address string) {
	c.vaultAddress = address
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
func (c *Client) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
	body, err := c.buildPayload(payload, c.vaultAddress, c.nonce.GetInt64())
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	pathURL := c.BaseURL.ResolveReference(rel)
	rawQuery := params.Encode()
	if rawQuery != "" {
		pathURL.RawQuery = rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	return req, nil
}

func (c *Client) SignL1Action(action any, timestamp int64, expiresAfter *int64) (SignatureResult, error) {
	data, err := c.buildActionData(action, uint64(timestamp), c.vaultAddress, expiresAfter)
	if err != nil {
		return SignatureResult{}, err
	}

	phantomAgent := c.PhantomAgent(crypto.Keccak256(data))
	chainId := math.HexOrDecimal256(*big.NewInt(1337))
	return c.sign(apitypes.TypedData{
		Domain: apitypes.TypedDataDomain{
			ChainId:           &chainId,
			Name:              "Exchange",
			Version:           "1",
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Types: apitypes.Types{
			"Agent": []apitypes.Type{
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
		},
		PrimaryType: "Agent",
		Message:     phantomAgent,
	}, c.privateKey)
}

func (c *Client) PhantomAgent(hash []byte) map[string]any {
	source := "b" // testnet
	if !TestNet {
		source = "a" // mainnet
	}

	return map[string]any{
		"source":       source,
		"connectionId": hash,
	}
}

func (c *Client) sign(typedData apitypes.TypedData, privateKey *ecdsa.PrivateKey) (SignatureResult, error) {
	// Create EIP-712 hash
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return SignatureResult{}, fmt.Errorf("failed to hash domain: %w", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return SignatureResult{}, fmt.Errorf("failed to hash typed data: %w", err)
	}

	data := []byte{0x19, 0x01}
	data = append(data, domainSeparator...)
	data = append(data, typedDataHash...)

	signature, err := crypto.Sign(crypto.Keccak256(data), privateKey)
	if err != nil {
		return SignatureResult{}, fmt.Errorf("failed to sign message: %w", err)
	}

	// Extract r, s, v components
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])
	v := int(signature[64]) + 27

	return SignatureResult{
		R: hexutil.EncodeBig(r),
		S: hexutil.EncodeBig(s),
		V: v,
	}, nil
}

// buildActionData constructs the data for action hashing
func (c *Client) buildActionData(action any, nonce uint64, vaultAddress string, expiresAfter *int64) ([]byte, error) {
	// Marshal action to msgpack
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetSortMapKeys(true)
	enc.UseCompactInts(true)

	if err := enc.Encode(action); err != nil {
		return nil, fmt.Errorf("failed to marshal action: %v", err)
	}

	data := buf.Bytes()

	// Append nonce
	data = appendUint64(data, nonce)

	// Append vault address flag and address if present
	if vaultAddress == "" {
		data = append(data, 0x00)
	} else {
		data = append(data, 0x01)
		data = append(data, common.HexToAddress(vaultAddress).Bytes()...)
	}

	// Append expiration if provided
	if expiresAfter != nil {
		if *expiresAfter < 0 {
			return nil, fmt.Errorf("expiresAfter cannot be negative: %d", *expiresAfter)
		}
		data = append(data, 0x00)
		data = appendUint64(data, uint64(*expiresAfter))
	}

	return data, nil
}

func (c *Client) buildPayload(action any, vaultAddress string, nonce int64) ([]byte, error) {
	signature, err := c.SignL1Action(action, nonce, nil)
	if err != nil {
		return nil, err
	}

	// Marshal action to JSON
	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": signature,
	}

	if vaultAddress != "" {
		// Handle vault address based on action type
		if actionMap, ok := action.(map[string]any); ok {
			if actionMap["type"] != "usdClassTransfer" {
				payload["vaultAddress"] = vaultAddress
			} else {
				payload["vaultAddress"] = nil
			}
		} else {
			// For struct types, we need to use reflection or type assertion
			// For now, assume it's not usdClassTransfer
			payload["vaultAddress"] = vaultAddress
		}
	}

	return json.Marshal(payload)
}

// appendUint64 appends a uint64 as 8 bytes in big-endian format
func appendUint64(data []byte, value uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, value)
	return append(data, bytes...)
}

func getAPIEndpoint() string {
	if TestNet {
		return TestNetURL
	}
	return ProductionURL
}
