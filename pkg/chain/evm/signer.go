package evm

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer signs transactions and arbitrary hashes for one address. The local
// private key implementation is the only one for now; a KMS or hardware wallet
// signer implements the same interface.
type Signer interface {
	Address() common.Address

	SignTx(tx *ethtypes.Transaction, chainID *big.Int) (*ethtypes.Transaction, error)

	SignHash(hash common.Hash) ([]byte, error)
}

// PrivateKeySigner signs with an in-process secp256k1 private key.
type PrivateKeySigner struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// NewPrivateKeySigner parses a hex-encoded private key, with or without the 0x
// prefix. The key itself is never logged or exposed by this type.
func NewPrivateKeySigner(hexKey string) (*PrivateKeySigner, error) {
	hexKey = strings.TrimSpace(hexKey)
	hexKey = strings.TrimPrefix(hexKey, "0x")
	hexKey = strings.TrimPrefix(hexKey, "0X")

	if hexKey == "" {
		return nil, errors.New("private key is empty")
	}

	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		// the error from HexToECDSA does not contain the key, but be explicit
		return nil, errors.New("failed to parse the private key: invalid hex-encoded secp256k1 key")
	}

	return &PrivateKeySigner{
		key:     key,
		address: crypto.PubkeyToAddress(key.PublicKey),
	}, nil
}

func (s *PrivateKeySigner) Address() common.Address {
	return s.address
}

func (s *PrivateKeySigner) SignTx(tx *ethtypes.Transaction, chainID *big.Int) (*ethtypes.Transaction, error) {
	if chainID == nil {
		return nil, errors.New("chain id is required for signing")
	}

	return ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), s.key)
}

func (s *PrivateKeySigner) SignHash(hash common.Hash) ([]byte, error) {
	return crypto.Sign(hash.Bytes(), s.key)
}

// String intentionally exposes only the address.
func (s *PrivateKeySigner) String() string {
	return fmt.Sprintf("PrivateKeySigner(%s)", s.address.Hex())
}
