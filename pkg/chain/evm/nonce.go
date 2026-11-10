package evm

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

// NonceReader is the subset of the client that the nonce manager needs.
type NonceReader interface {
	PendingNonceAt(ctx context.Context, address common.Address) (uint64, error)
}

// NonceManager hands out the next nonce for one address on one chain.
//
// It synchronizes lazily from the node's pending pool and then counts locally,
// because asking the node for every transaction races with the node's own view
// of the pending pool and differs between RPC providers. Nothing is persisted:
// on restart the first allocation resynchronizes from the chain.
type NonceManager struct {
	reader  NonceReader
	address common.Address

	mu     sync.Mutex
	next   uint64
	synced bool
}

func NewNonceManager(reader NonceReader, address common.Address) *NonceManager {
	return &NonceManager{reader: reader, address: address}
}

func (m *NonceManager) Address() common.Address {
	return m.address
}

// Next allocates the next nonce. The caller must either broadcast a transaction
// with it, or hand it back with Release.
func (m *NonceManager) Next(ctx context.Context) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.synced {
		if err := m.syncLocked(ctx); err != nil {
			return 0, err
		}
	}

	nonce := m.next
	m.next++

	return nonce, nil
}

// Release returns an unused nonce, which only works for the most recently
// allocated one. Anything else means transactions were allocated out of order
// and the manager resynchronizes instead of guessing.
func (m *NonceManager) Release(nonce uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.synced && m.next == nonce+1 {
		m.next = nonce
		return
	}

	log.Warnf("evm: released nonce %d is not the last allocated one, resynchronizing", nonce)
	m.synced = false
}

// Reset forces the next allocation to resynchronize from the chain. Call it
// after a broadcast fails in a way that makes the local count unreliable, such
// as "nonce too low".
func (m *NonceManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = false
}

// Sync reloads the next nonce from the node's pending pool.
func (m *NonceManager) Sync(ctx context.Context) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.syncLocked(ctx); err != nil {
		return 0, err
	}

	return m.next, nil
}

func (m *NonceManager) syncLocked(ctx context.Context) error {
	nonce, err := m.reader.PendingNonceAt(ctx, m.address)
	if err != nil {
		return fmt.Errorf("failed to read the pending nonce of %s: %w", m.address.Hex(), err)
	}

	m.next = nonce
	m.synced = true

	return nil
}
