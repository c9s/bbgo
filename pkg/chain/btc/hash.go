package btc

import (
	"encoding/hex"
	"fmt"
)

// HashSize is the size of a block hash or transaction id, a double SHA-256.
const HashSize = 32

// Hash is a block hash or a transaction id, kept in the byte order of the
// wire format: the bytes as the double SHA-256 produced them.
//
// Bitcoin Core, block explorers and every RPC show these hashes byte-reversed,
// so String, MarshalText and NewHashFromString all work with the reversed
// form. Never hex-encode the array directly to compare it with an RPC value.
type Hash [HashSize]byte

// NewHashFromString parses a hash in the byte-reversed hex form used by the RPC.
func NewHashFromString(s string) (Hash, error) {
	var h Hash
	if len(s) != HashSize*2 {
		return h, fmt.Errorf("btc: hash %q must be %d hex characters", s, HashSize*2)
	}

	var b [HashSize]byte
	if _, err := hex.Decode(b[:], []byte(s)); err != nil {
		return h, fmt.Errorf("btc: invalid hash %q: %w", s, err)
	}

	for i := range b {
		h[i] = b[HashSize-1-i]
	}

	return h, nil
}

// MustHash is NewHashFromString for constants; it panics on an invalid hash.
func MustHash(s string) Hash {
	h, err := NewHashFromString(s)
	if err != nil {
		panic(err)
	}

	return h
}

// String returns the byte-reversed hex form used by the RPC and explorers.
func (h Hash) String() string {
	var b [HashSize]byte
	for i := range h {
		b[i] = h[HashSize-1-i]
	}

	return hex.EncodeToString(b[:])
}

// IsZero reports whether h is all zeros, which is the previous output of a
// coinbase input.
func (h Hash) IsZero() bool {
	return h == Hash{}
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

func (h *Hash) UnmarshalText(text []byte) error {
	parsed, err := NewHashFromString(string(text))
	if err != nil {
		return err
	}

	*h = parsed
	return nil
}
