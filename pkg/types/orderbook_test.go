package types

import (
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// orderBookBackend names an OrderBook implementation and how to construct an empty one,
// so every benchmark below can run the identical workload against all three backends.
type orderBookBackend struct {
	name string
	new  func(symbol string) OrderBook
}

var orderBookBackends = []orderBookBackend{
	{"SliceOrderBook", func(symbol string) OrderBook { return NewSliceOrderBook(symbol) }},
	{"RBTOrderBook", func(symbol string) OrderBook { return NewRBOrderBook(symbol) }},
	{"SkipListOrderBook", func(symbol string) OrderBook { return NewSkipListOrderBook(symbol) }},
}

// orderBookDepths are the book sizes (price levels per side) the benchmarks run at.
// Real books on liquid pairs sit in the thousands of levels, so the small-book case is
// kept only as a baseline for comparison against the large ones.
var orderBookDepths = []int{1000, 5000, 8000}

const benchmarkMidPrice = 30000.0

// benchmarkTick is the price increment between two adjacent levels. At 8000 levels per
// side this spans 80 price units around the mid price, which is a realistic tick density.
const benchmarkTick = 0.01

// makeOrderBookSnapshot builds a full book snapshot with the given number of price levels
// per side. Bids descend from just below the mid price and asks ascend from just above it,
// which is the order both the slice and the tree backends expect from a snapshot.
func makeOrderBookSnapshot(symbol string, levels int) SliceOrderBook {
	book := SliceOrderBook{
		Symbol: symbol,
		Bids:   make(PriceVolumeSlice, 0, levels),
		Asks:   make(PriceVolumeSlice, 0, levels),
	}

	for i := 0; i < levels; i++ {
		offset := float64(i) * benchmarkTick
		book.Bids = append(book.Bids, PriceVolume{
			Price:  fixedpoint.NewFromFloat(benchmarkMidPrice - benchmarkTick - offset),
			Volume: fixedpoint.One,
		})
		book.Asks = append(book.Asks, PriceVolume{
			Price:  fixedpoint.NewFromFloat(benchmarkMidPrice + benchmarkTick + offset),
			Volume: fixedpoint.One,
		})
	}

	return book
}

// benchmarkDeltaLevels is the number of price levels touched by a single incremental
// update, roughly the size of one websocket diff message.
const benchmarkDeltaLevels = 20

// benchmarkDeltaBatches is how many distinct pre-generated update batches are cycled
// through, so the benchmark does not repeat the exact same write over and over.
const benchmarkDeltaBatches = 64

// makeVolumeDeltas pre-generates update batches that only change the volume of existing
// price levels. The book size never changes, so the measurement stays stable no matter
// how many iterations the benchmark runs.
func makeVolumeDeltas(symbol string, snapshot SliceOrderBook) []SliceOrderBook {
	rnd := rand.New(rand.NewSource(1))
	levels := len(snapshot.Bids)

	deltas := make([]SliceOrderBook, 0, benchmarkDeltaBatches)
	for i := 0; i < benchmarkDeltaBatches; i++ {
		delta := SliceOrderBook{
			Symbol: symbol,
			Bids:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
			Asks:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
		}

		for j := 0; j < benchmarkDeltaLevels; j++ {
			// bias towards the top of the book, which is where real updates cluster
			idx := rnd.Intn(levels) * rnd.Intn(levels) / levels
			volume := fixedpoint.NewFromFloat(1 + rnd.Float64())
			delta.Bids = append(delta.Bids, PriceVolume{Price: snapshot.Bids[idx].Price, Volume: volume})
			delta.Asks = append(delta.Asks, PriceVolume{Price: snapshot.Asks[idx].Price, Volume: volume})
		}

		// a delta is applied in book order by every backend, so sort it the same way
		sort.Slice(delta.Bids, func(a, b int) bool { return delta.Bids[a].Price.Compare(delta.Bids[b].Price) > 0 })
		sort.Slice(delta.Asks, func(a, b int) bool { return delta.Asks[a].Price.Compare(delta.Asks[b].Price) < 0 })
		deltas = append(deltas, delta)
	}

	return deltas
}

// makeChurnDeltas pre-generates update batches that delete price levels and then put them
// back. Batches alternate between a delete pass (zero volume) and the matching restore
// pass, so the book oscillates between two sizes instead of growing or draining without
// bound over the course of the benchmark.
func makeChurnDeltas(symbol string, snapshot SliceOrderBook) []SliceOrderBook {
	rnd := rand.New(rand.NewSource(2))
	levels := len(snapshot.Bids)

	deltas := make([]SliceOrderBook, 0, benchmarkDeltaBatches)
	for i := 0; i < benchmarkDeltaBatches; i += 2 {
		remove := SliceOrderBook{
			Symbol: symbol,
			Bids:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
			Asks:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
		}
		restore := SliceOrderBook{
			Symbol: symbol,
			Bids:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
			Asks:   make(PriceVolumeSlice, 0, benchmarkDeltaLevels),
		}

		for j := 0; j < benchmarkDeltaLevels; j++ {
			idx := rnd.Intn(levels)
			bid, ask := snapshot.Bids[idx], snapshot.Asks[idx]
			remove.Bids = append(remove.Bids, PriceVolume{Price: bid.Price, Volume: fixedpoint.Zero})
			remove.Asks = append(remove.Asks, PriceVolume{Price: ask.Price, Volume: fixedpoint.Zero})
			restore.Bids = append(restore.Bids, bid)
			restore.Asks = append(restore.Asks, ask)
		}

		deltas = append(deltas, remove, restore)
	}

	return deltas
}

// benchmarkBackends runs fn against every backend at every configured depth, naming the
// sub-benchmarks "<Backend>/<levels>" so the results group per implementation.
func benchmarkBackends(b *testing.B, fn func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook)) {
	b.Helper()

	for _, backend := range orderBookBackends {
		b.Run(backend.name, func(b *testing.B) {
			for _, levels := range orderBookDepths {
				snapshot := makeOrderBookSnapshot("BTCUSDT", levels)
				b.Run(strconv.Itoa(levels), func(b *testing.B) {
					fn(b, backend, snapshot)
				})
			}
		})
	}
}

// BenchmarkOrderBook_Load measures loading a full snapshot into an empty book, which is
// what happens on every stream (re)connect.
func BenchmarkOrderBook_Load(b *testing.B) {
	benchmarkBackends(b, func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook) {
		book := backend.new(snapshot.Symbol)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			book.Load(snapshot)
		}
	})
}

// BenchmarkOrderBook_UpdateVolume measures applying an incremental update that only
// changes the volume of existing price levels. This is the hot path: it runs on every
// websocket diff message.
func BenchmarkOrderBook_UpdateVolume(b *testing.B) {
	benchmarkBackends(b, func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook) {
		deltas := makeVolumeDeltas(snapshot.Symbol, snapshot)
		book := backend.new(snapshot.Symbol)
		book.Load(snapshot)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			book.Update(deltas[i%len(deltas)])
		}
	})
}

// BenchmarkOrderBook_UpdateChurn measures applying an incremental update that removes and
// re-adds price levels, so the backend pays for structural changes rather than in-place
// volume writes.
func BenchmarkOrderBook_UpdateChurn(b *testing.B) {
	benchmarkBackends(b, func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook) {
		deltas := makeChurnDeltas(snapshot.Symbol, snapshot)
		book := backend.new(snapshot.Symbol)
		book.Load(snapshot)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			book.Update(deltas[i%len(deltas)])
		}
	})
}

// BenchmarkOrderBook_BestBidAsk measures reading the top of the book, which strategies do
// far more often than they write.
func BenchmarkOrderBook_BestBidAsk(b *testing.B) {
	benchmarkBackends(b, func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook) {
		book := backend.new(snapshot.Symbol)
		book.Load(snapshot)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			book.BestBid()
			book.BestAsk()
		}
	})
}

// BenchmarkOrderBook_CopyDepth measures materializing the top 20 levels of both sides,
// which is what a strategy does when it needs a stable view of the book.
//
// Note this uses CopyDepth rather than SideBook: SliceOrderBook.SideBook hands back its
// backing slice without copying or truncating, so comparing it against the tree backends
// (which materialize a bounded slice) would not measure the same work.
func BenchmarkOrderBook_CopyDepth(b *testing.B) {
	benchmarkBackends(b, func(b *testing.B, backend orderBookBackend, snapshot SliceOrderBook) {
		book := backend.new(snapshot.Symbol)
		book.Load(snapshot)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			book.CopyDepth(20)
		}
	})
}

func TestOrderBook_IsValid(t *testing.T) {
	ob := SliceOrderBook{
		Bids: PriceVolumeSlice{
			{fixedpoint.NewFromFloat(100.0), fixedpoint.NewFromFloat(1.5)},
			{fixedpoint.NewFromFloat(90.0), fixedpoint.NewFromFloat(2.5)},
		},

		Asks: PriceVolumeSlice{
			{fixedpoint.NewFromFloat(110.0), fixedpoint.NewFromFloat(1.5)},
			{fixedpoint.NewFromFloat(120.0), fixedpoint.NewFromFloat(2.5)},
		},
	}

	isValid, err := ob.IsValid()
	assert.True(t, isValid)
	assert.NoError(t, err)

	ob.Bids = nil
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "empty bids")

	ob.Bids = PriceVolumeSlice{
		{fixedpoint.NewFromFloat(80000.0), fixedpoint.NewFromFloat(1.5)},
		{fixedpoint.NewFromFloat(120.0), fixedpoint.NewFromFloat(2.5)},
	}

	ob.Asks = nil
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "empty asks")

	ob.Asks = PriceVolumeSlice{
		{fixedpoint.NewFromFloat(100.0), fixedpoint.NewFromFloat(1.5)},
		{fixedpoint.NewFromFloat(90.0), fixedpoint.NewFromFloat(2.5)},
	}
	isValid, err = ob.IsValid()
	assert.False(t, isValid)
	assert.EqualError(t, err, "bid price 80000 > ask price 100")
}
