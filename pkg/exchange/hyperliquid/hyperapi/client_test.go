package hyperapi

import (
	"context"
	"encoding/binary"
	"math/big"
	"net/url"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func TestNewClient(t *testing.T) {
	// Test default client
	client := NewClient()
	if client == nil {
		t.Error("NewClient should return a valid client")
	}
}

func TestBuildActionData(t *testing.T) {
	client := NewClient()

	action := map[string]interface{}{
		"type": "order",
		"data": "test",
	}

	data, err := client.buildActionData(action, 12345, "", nil)
	if err != nil {
		t.Errorf("buildActionData should not return error: %v", err)
	}
	if len(data) == 0 {
		t.Error("buildActionData should return non-empty data")
	}
}

func TestPhantomAgent(t *testing.T) {
	// Test production client phantom agent
	originalTestNet := TestNet
	TestNet = false
	defer func() { TestNet = originalTestNet }()

	client := NewClient()
	agent := client.PhantomAgent([]byte("test"))
	if agent["source"] != "a" {
		t.Errorf("Expected production source 'a', got %s", agent["source"])
	}

	// Test testnet client phantom agent
	TestNet = true
	client = NewClient()
	agent = client.PhantomAgent([]byte("test"))
	if agent["source"] != "b" {
		t.Errorf("Expected testnet source 'b', got %s", agent["source"])
	}
}

func TestAuth(t *testing.T) {
	client := NewClient()

	// Test with valid private key
	validPrivateKey, _ := crypto.GenerateKey()
	validPrivateKeyHex := hexutil.Encode(crypto.FromECDSA(validPrivateKey))[2:] // Remove 0x prefix

	// Should not panic with valid key
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Auth should not panic with valid private key: %v", r)
			}
		}()
		client.Auth(validPrivateKeyHex)
	}()

	// Verify private key was set
	if client.privateKey == nil {
		t.Error("Private key should be set after Auth")
	}

	// Test with invalid private key
	client2 := NewClient()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Auth should panic with invalid private key")
			}
		}()
		client2.Auth("invalid_private_key")
	}()
}

func TestSetVaultAddress(t *testing.T) {
	client := NewClient()

	// Test setting vault address
	vaultAddr := "0x1234567890123456789012345678901234567890"
	client.SetVaultAddress(vaultAddr)

	if client.vaultAddress != vaultAddr {
		t.Errorf("Expected vault address %s, got %s", vaultAddr, client.vaultAddress)
	}
}

func TestSingL1Action(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, _ := crypto.GenerateKey()
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	client.Auth(privateKeyHex)

	action := map[string]interface{}{
		"type": "order",
		"data": "test",
	}

	// Test signing without expiration
	signature, err := client.SignL1Action(action, 12345, nil)
	if err != nil {
		t.Errorf("SignL1Action should not return error: %v", err)
	}

	// Verify signature components
	if signature.R == "" || signature.S == "" || signature.V == 0 {
		t.Error("Signature should have valid R, S, V components")
	}

	// Test signing with expiration
	expiresAfter := int64(3600)
	signature2, err := client.SignL1Action(action, 12345, &expiresAfter)
	if err != nil {
		t.Errorf("SignL1Action with expiration should not return error: %v", err)
	}

	// Signatures should be different
	if signature.R == signature2.R && signature.S == signature2.S {
		t.Error("Signatures with different expiration should be different")
	}
}

func TestSign(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, _ := crypto.GenerateKey()
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	client.Auth(privateKeyHex)

	// Create test typed data
	typedData := apitypes.TypedData{
		Domain: apitypes.TypedDataDomain{
			Name:              "Test",
			Version:           "1",
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(1)),
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Types: apitypes.Types{
			"Test": []apitypes.Type{
				{Name: "test", Type: "string"},
			},
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
		},
		PrimaryType: "Test",
		Message: map[string]interface{}{
			"test": "value",
		},
	}

	signature, err := client.sign(typedData, privateKey)
	if err != nil {
		t.Errorf("sign should not return error: %v", err)
	}

	// Verify signature components
	if signature.R == "" || signature.S == "" || signature.V == 0 {
		t.Error("Signature should have valid R, S, V components")
	}
}

func TestBuildActionDataWithVault(t *testing.T) {
	client := NewClient()
	client.SetVaultAddress("0x1234567890123456789012345678901234567890")

	action := map[string]interface{}{
		"type": "order",
		"data": "test",
	}

	// Test with vault address
	data, err := client.buildActionData(action, 12345, "", nil)
	if err != nil {
		t.Errorf("buildActionData with vault should not return error: %v", err)
	}
	if len(data) == 0 {
		t.Error("buildActionData should return non-empty data")
	}

	// Test with expiration
	expiresAfter := int64(3600)
	data2, err := client.buildActionData(action, 12345, "", &expiresAfter)
	if err != nil {
		t.Errorf("buildActionData with expiration should not return error: %v", err)
	}
	if len(data2) == 0 {
		t.Error("buildActionData with expiration should return non-empty data")
	}

	// Data with expiration should be different
	if string(data) == string(data2) {
		t.Error("Data with expiration should be different from data without expiration")
	}
}

func TestNewAuthenticatedRequest(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, _ := crypto.GenerateKey()
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	client.Auth(privateKeyHex)

	action := map[string]interface{}{
		"type": "order",
		"data": "test",
	}

	ctx := context.Background()
	params := url.Values{}
	params.Set("test", "value")

	req, err := client.NewAuthenticatedRequest(ctx, "POST", "/test", params, action)
	if err != nil {
		t.Errorf("NewAuthenticatedRequest should not return error: %v", err)
	}

	if req == nil {
		t.Error("NewAuthenticatedRequest should return a valid request")
		return
	}

	// Verify request headers
	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("Request should have correct Content-Type header")
	}
	if req.Header.Get("Accept") != "application/json" {
		t.Error("Request should have correct Accept header")
	}
}

func TestBuildPayload(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, _ := crypto.GenerateKey()
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	client.Auth(privateKeyHex)

	// Test with regular action
	action := map[string]interface{}{
		"type": "order",
		"data": "test",
	}

	payload, err := client.buildPayload(action, client.vaultAddress, client.nonce.GetInt64())
	if err != nil {
		t.Errorf("castPayload should not return error: %v", err)
	}

	if len(payload) == 0 {
		t.Error("castPayload should return non-empty payload")
	}

	// Test with usdClassTransfer action (should not include vault address)
	client.SetVaultAddress("0x1234567890123456789012345678901234567890")
	usdTransferAction := map[string]interface{}{
		"type": "usdClassTransfer",
		"data": "test",
	}

	payload2, err := client.buildPayload(usdTransferAction, client.vaultAddress, client.nonce.GetInt64())
	if err != nil {
		t.Errorf("castPayload with usdClassTransfer should not return error: %v", err)
	}

	if len(payload2) == 0 {
		t.Error("castPayload with usdClassTransfer should return non-empty payload")
	}
}

func TestAppendUint64(t *testing.T) {
	// Test appendUint64 function
	data := []byte("test")
	value := uint64(12345)

	result := appendUint64(data, value)

	// Should be 8 bytes longer
	if len(result) != len(data)+8 {
		t.Errorf("Expected length %d, got %d", len(data)+8, len(result))
	}

	// Verify the appended value
	appendedValue := binary.BigEndian.Uint64(result[len(data):])
	if appendedValue != value {
		t.Errorf("Expected value %d, got %d", value, appendedValue)
	}
}

func TestGetAPIEndpoint(t *testing.T) {
	// Test production endpoint
	originalTestNet := TestNet
	TestNet = false
	defer func() { TestNet = originalTestNet }()

	endpoint := getAPIEndpoint()
	if endpoint != ProductionURL {
		t.Errorf("Expected production URL %s, got %s", ProductionURL, endpoint)
	}

	// Test testnet endpoint
	TestNet = true
	endpoint = getAPIEndpoint()
	if endpoint != TestNetURL {
		t.Errorf("Expected testnet URL %s, got %s", TestNetURL, endpoint)
	}
}
