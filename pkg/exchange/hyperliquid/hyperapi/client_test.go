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
	"github.com/vmihailenco/msgpack/v5"
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
		"type": ReqSubmitOrder,
		"data": "test",
	}

	hash, err := client.buildActionHash(action, 12345, "", nil)
	if err != nil {
		t.Errorf("buildActionData should not return error: %v", err)
	}
	if len(hash) == 0 {
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
	account := crypto.PubkeyToAddress(validPrivateKey.PublicKey).Hex()

	// Should not panic with valid key
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Auth should not panic with valid private key: %v", r)
			}
		}()
		client.Auth(validPrivateKeyHex, account)
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
		client2.Auth("invalid_private_key", "")
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
	account := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	client.Auth(privateKeyHex, account)

	action := map[string]interface{}{
		"type": ReqSubmitOrder,
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
	account := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	client.Auth(privateKeyHex, account)

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
		"type": ReqSubmitOrder,
		"data": "test",
	}

	// Test with vault address
	hash, err := client.buildActionHash(action, 12345, "", nil)
	if err != nil {
		t.Errorf("buildActionData with vault should not return error: %v", err)
	}
	if len(hash) == 0 {
		t.Error("buildActionData should return non-empty data")
	}

	// Test with expiration
	expiresAfter := int64(3600)
	hash2, err := client.buildActionHash(action, 12345, "", &expiresAfter)
	if err != nil {
		t.Errorf("buildActionData with expiration should not return error: %v", err)
	}
	if len(hash2) == 0 {
		t.Error("buildActionData with expiration should return non-empty data")
	}

	// Data with expiration should be different
	if string(hash) == string(hash2) {
		t.Error("Data with expiration should be different from data without expiration")
	}
}

func TestNewAuthenticatedRequest(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, _ := crypto.GenerateKey()
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	account := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	client.Auth(privateKeyHex, account)

	action := map[string]interface{}{
		"type": ReqSubmitOrder,
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
	account := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	client.Auth(privateKeyHex, account)

	// Test with regular action
	action := map[string]interface{}{
		"type": ReqSubmitOrder,
		"data": "test",
	}

	payload, err := client.buildPayload(action, client.vaultAddress, client.nonce.GetInt64())
	if err != nil {
		t.Errorf("castPayload should not return error: %v", err)
	}

	if len(payload) == 0 {
		t.Error("castPayload should return non-empty payload")
	}

	// TODO add test case
	// Test with usdClassTransfer action (should not include vault address)
	// client.SetVaultAddress("0x1234567890123456789012345678901234567890")
	// usdTransferAction := map[string]interface{}{
	// 	"type": "usdClassTransfer",
	// 	"data": "test",
	// }

	// payload2, err := client.buildPayload(usdTransferAction, client.vaultAddress, client.nonce.GetInt64())
	// if err != nil {
	// 	t.Errorf("castPayload with usdClassTransfer should not return error: %v", err)
	// }

	// if len(payload2) == 0 {
	// 	t.Error("castPayload with usdClassTransfer should return non-empty payload")
	// }
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

func TestConvertToAction(t *testing.T) {
	client := NewClient()

	t.Run("OrderAction", func(t *testing.T) {
		// Test order action conversion
		action := map[string]any{
			"type": ReqSubmitOrder,
			"orders": []map[string]any{
				{
					"a": 0,
					"b": true,
					"p": "100000",
					"s": "0.001",
					"r": false,
					"t": map[string]any{
						"limit": map[string]any{
							"tif": "Gtc",
						},
					},
				},
			},
			"grouping": "na",
		}

		packed, err := client.convertToAction(action)
		if err != nil {
			t.Fatalf("convertToAction should not return error: %v", err)
		}

		if len(packed) == 0 {
			t.Error("convertToAction should return non-empty packed data")
		}

		// Verify it's valid msgpack
		var decoded OrderAction
		err = msgpack.Unmarshal(packed, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal packed data: %v", err)
		}

		if decoded.Type != "order" {
			t.Errorf("Expected type 'order', got %s", decoded.Type)
		}

		if len(decoded.Orders) != 1 {
			t.Errorf("Expected 1 order, got %d", len(decoded.Orders))
		}

		if decoded.Orders[0].Asset != 0 {
			t.Errorf("Expected asset 0, got %d", decoded.Orders[0].Asset)
		}

		if !decoded.Orders[0].IsBuy {
			t.Error("Expected IsBuy to be true")
		}

		if decoded.Orders[0].LimitPx != "100000" {
			t.Errorf("Expected LimitPx '100000', got %s", decoded.Orders[0].LimitPx)
		}
	})

	t.Run("UnsupportedActionType", func(t *testing.T) {
		action := map[string]any{
			"type": "unsupported",
		}

		_, err := client.convertToAction(action)
		if err == nil {
			t.Error("convertToAction should return error for unsupported action type")
		}
	})

	t.Run("InvalidActionType", func(t *testing.T) {
		// Test with non-map action
		_, err := client.convertToAction("not a map")
		if err == nil {
			t.Error("convertToAction should return error for non-map action")
		}
	})
}

// TestSignatureAddressRecovery verifies that the address recovered from signature matches the original address
func TestSignatureAddressRecovery(t *testing.T) {
	client := NewClient()

	// Generate a valid private key for testing
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))[2:] // Remove 0x prefix
	account := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	client.Auth(privateKeyHex, account)

	// Get the original address from the private key
	originalAddress := client.UserAddress()
	if originalAddress == "" {
		t.Fatal("UserAddress should return a valid address")
	}

	// Create a test action
	action := map[string]any{
		"type": ReqSubmitOrder,
		"orders": []map[string]any{
			{
				"a": 0,
				"b": true,
				"p": "100000",
				"s": "0.001",
				"r": false,
				"t": map[string]any{
					"limit": map[string]any{
						"tif": "Gtc",
					},
				},
			},
		},
		"grouping": "na",
	}

	nonce := int64(1234567890)

	// Sign the action
	signature, err := client.SignL1Action(action, nonce, nil)
	if err != nil {
		t.Fatalf("SignL1Action should not return error: %v", err)
	}

	// Rebuild the action data to get the hash (same as in SignL1Action)
	hash, err := client.buildActionHash(action, uint64(nonce), client.vaultAddress, nil)
	if err != nil {
		t.Fatalf("buildActionData should not return error: %v", err)
	}

	// Rebuild the PhantomAgent (same as in SignL1Action)
	phantomAgent := client.PhantomAgent(hash)
	chainId := math.HexOrDecimal256(*big.NewInt(1337))

	// Rebuild the typed data (same as in SignL1Action)
	typedData := apitypes.TypedData{
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
	}

	recoverHash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatalf("TypedDataAndHash should not return error: %v", err)
	}

	sigBytes := make([]byte, 65)
	copy(sigBytes[:32], hexutil.MustDecode(signature.R))
	copy(sigBytes[32:64], hexutil.MustDecode(signature.S))
	sigBytes[64] = byte(signature.V - 27)

	pub, _ := crypto.SigToPub(recoverHash, sigBytes)
	recoveredAddress := crypto.PubkeyToAddress(*pub).Hex()

	// Verify that recovered address matches original address
	if recoveredAddress != originalAddress {
		t.Errorf("Recovered address %s does not match original address %s", recoveredAddress, originalAddress)
	} else {
		t.Logf("Successfully verified: recovered address %s matches original address %s", recoveredAddress, originalAddress)
	}
}
