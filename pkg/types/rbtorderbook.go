package types

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate callbackgen -type RBTOrderBook
type RBTOrderBook struct {
	Symbol string
	Bids   *RBTree
	Asks   *RBTree

	lastUpdateTime time.Time

	loadCallbacks   []func(book *RBTOrderBook)
	updateCallbacks []func(book *RBTOrderBook)
}

func NewRBOrderBook(symbol string) *RBTOrderBook {
	return &RBTOrderBook{
		Symbol: symbol,
		Bids:   NewRBTree(),
		Asks:   NewRBTree(),
	}
}

func (b *RBTOrderBook) LastUpdateTime() time.Time {
	return b.lastUpdateTime
}

func (b *RBTOrderBook) BestBid() (PriceVolume, bool) {
	right := b.Bids.Rightmost()
	if right != nil {
		return PriceVolume{Price: right.key, Volume: right.value}, true
	}

	return PriceVolume{}, false
}

func (b *RBTOrderBook) BestAsk() (PriceVolume, bool) {
	left := b.Asks.Leftmost()
	if left != nil {
		return PriceVolume{Price: left.key, Volume: left.value}, true
	}

	return PriceVolume{}, false
}

func (b *RBTOrderBook) Spread() (fixedpoint.Value, bool) {
	bestBid, ok := b.BestBid()
	if !ok {
		return fixedpoint.Zero, false
	}

	bestAsk, ok := b.BestAsk()
	if !ok {
		return fixedpoint.Zero, false
	}

	return bestAsk.Price.Sub(bestBid.Price), true
}

func (b *RBTOrderBook) IsValid() (bool, error) {
	bid, hasBid := b.BestBid()
	ask, hasAsk := b.BestAsk()

	if !hasBid {
		return false, errors.New("empty bids")
	}

	if !hasAsk {
		return false, errors.New("empty asks")
	}

	if bid.Price.Compare(ask.Price) > 0 {
		return false, fmt.Errorf("bid price %s > ask price %s", bid.Price.String(), ask.Price.String())
	}

	return true, nil
}

func (b *RBTOrderBook) Load(book SliceOrderBook) {
	b.Reset()
	b.update(book)
	b.EmitLoad(b)
}

func (b *RBTOrderBook) Update(book SliceOrderBook) {
	b.update(book)
	b.EmitUpdate(b)
}

func (b *RBTOrderBook) Reset() {
	b.Bids = NewRBTree()
	b.Asks = NewRBTree()
}

func (b *RBTOrderBook) updateAsks(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume.IsZero() {
			b.Asks.Delete(pv.Price)
		} else {
			b.Asks.Upsert(pv.Price, pv.Volume)
		}
	}
}

func (b *RBTOrderBook) updateBids(pvs PriceVolumeSlice) {
	for _, pv := range pvs {
		if pv.Volume.IsZero() {
			b.Bids.Delete(pv.Price)
		} else {
			b.Bids.Upsert(pv.Price, pv.Volume)
		}
	}
}

func (b *RBTOrderBook) update(book SliceOrderBook) {
	b.updateBids(book.Bids)
	b.updateAsks(book.Asks)
	b.lastUpdateTime = time.Now()
}

func (b *RBTOrderBook) load(book SliceOrderBook) {
	b.Reset()
	b.updateBids(book.Bids)
	b.updateAsks(book.Asks)
	b.lastUpdateTime = time.Now()
}

func (b *RBTOrderBook) Copy() OrderBook {
	var book = NewRBOrderBook(b.Symbol)
	book.Asks = b.Asks.CopyInorder(0)
	book.Bids = b.Bids.CopyInorder(0)
	return book
}

func (b *RBTOrderBook) CopyDepth(limit int) OrderBook {
	var book = NewRBOrderBook(b.Symbol)
	book.Asks = b.Asks.CopyInorder(limit)
	book.Bids = b.Bids.CopyInorderReverse(limit)
	return book
}

func (b *RBTOrderBook) convertTreeToPriceVolumeSlice(tree *RBTree, limit int, descending bool) (pvs PriceVolumeSlice) {
	if descending {
		tree.InorderReverse(func(n *RBNode) bool {
			pvs = append(pvs, PriceVolume{
				Price:  n.key,
				Volume: n.value,
			})

			return !(limit > 0 && len(pvs) >= limit)
		})

		return pvs
	}

	tree.Inorder(func(n *RBNode) bool {
		pvs = append(pvs, PriceVolume{
			Price:  n.key,
			Volume: n.value,
		})

		return !(limit > 0 && len(pvs) >= limit)
	})
	return pvs
}

func (b *RBTOrderBook) SideBook(sideType SideType) PriceVolumeSlice {
	switch sideType {

	case SideTypeBuy:
		return b.convertTreeToPriceVolumeSlice(b.Bids, 0, true)

	case SideTypeSell:
		return b.convertTreeToPriceVolumeSlice(b.Asks, 0, false)

	default:
		return nil
	}
}

func (b *RBTOrderBook) Print() {
	b.Asks.Inorder(func(n *RBNode) bool {
		fmt.Printf("ask: %s x %s", n.key.String(), n.value.String())
		return true
	})

	b.Bids.InorderReverse(func(n *RBNode) bool {
		fmt.Printf("bid: %s x %s", n.key.String(), n.value.String())
		return true
	})
}
