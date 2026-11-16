package evm

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type fakeNonceReader struct {
	nonce uint64
	err   error
	calls int
}

func (f *fakeNonceReader) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	f.calls++

	if f.err != nil {
		return 0, f.err
	}

	return f.nonce, nil
}

var testAddress = common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")

func TestNonceManagerNext(t *testing.T) {
	ctx := context.Background()
	reader := &fakeNonceReader{nonce: 7}
	manager := NewNonceManager(reader, testAddress)

	for expected := uint64(7); expected < 10; expected++ {
		nonce, err := manager.Next(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, nonce)
	}

	// the pending nonce is read once and then counted locally
	assert.Equal(t, 1, reader.calls)
}

func TestNonceManagerRelease(t *testing.T) {
	ctx := context.Background()
	reader := &fakeNonceReader{nonce: 3}
	manager := NewNonceManager(reader, testAddress)

	nonce, err := manager.Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), nonce)

	// releasing the last allocated nonce hands it back
	manager.Release(nonce)

	nonce, err = manager.Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), nonce)
	assert.Equal(t, 1, reader.calls)

	// releasing anything else cannot be trusted, so it resynchronizes
	manager.Release(99)
	reader.nonce = 11

	nonce, err = manager.Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(11), nonce)
	assert.Equal(t, 2, reader.calls)
}

func TestNonceManagerReset(t *testing.T) {
	ctx := context.Background()
	reader := &fakeNonceReader{nonce: 1}
	manager := NewNonceManager(reader, testAddress)

	_, err := manager.Next(ctx)
	assert.NoError(t, err)

	manager.Reset()
	reader.nonce = 42

	nonce, err := manager.Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(42), nonce)
}

func TestNonceManagerError(t *testing.T) {
	reader := &fakeNonceReader{err: errors.New("rpc down")}
	manager := NewNonceManager(reader, testAddress)

	_, err := manager.Next(context.Background())
	assert.ErrorContains(t, err, "rpc down")
	assert.ErrorContains(t, err, testAddress.Hex())
}
