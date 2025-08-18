package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Quota struct {
	mu        sync.Mutex
	Available fixedpoint.Value
	Locked    fixedpoint.Value
}

// Add adds the fund to the available quota
func (q *Quota) Add(fund fixedpoint.Value) {
	q.mu.Lock()
	q.Available = q.Available.Add(fund)
	q.mu.Unlock()
}

// Lock locks the fund from the available quota
// returns true if the fund is locked successfully
// returns false if the fund is not enough
func (q *Quota) Lock(fund fixedpoint.Value) bool {
	if fund.Compare(q.Available) > 0 {
		return false
	}

	q.mu.Lock()
	q.Available = q.Available.Sub(fund)
	q.Locked = q.Locked.Add(fund)
	q.mu.Unlock()

	return true
}

// Commit commits the locked fund
func (q *Quota) Commit() {
	q.mu.Lock()
	q.Locked = fixedpoint.Zero
	q.mu.Unlock()
}

// Rollback rolls back the locked fund
// this will move the locked fund to the available quota
func (q *Quota) Rollback() {
	q.mu.Lock()
	q.Available = q.Available.Add(q.Locked)
	q.Locked = fixedpoint.Zero
	q.mu.Unlock()
}

func (q *Quota) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.Locked.String() + "/" + q.Available.String()
}

// QuotaTransaction is a transactional quota manager
type QuotaTransaction struct {
	mu         sync.Mutex
	BaseAsset  Quota
	QuoteAsset Quota
}

// Commit commits the transaction
func (m *QuotaTransaction) Commit() bool {
	m.mu.Lock()
	m.BaseAsset.Commit()
	m.QuoteAsset.Commit()
	m.mu.Unlock()
	return true
}

// Rollback rolls back the transaction
func (m *QuotaTransaction) Rollback() bool {
	m.mu.Lock()
	m.BaseAsset.Rollback()
	m.QuoteAsset.Rollback()
	m.mu.Unlock()
	return true
}
