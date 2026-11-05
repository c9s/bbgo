package cloudsync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

const fixture = "testdata/order-book-updates-binance-BTCUSDT-trimmed.parquet"

// TestBookReader_RealSample decodes a trimmed slice of AmberData's public
// Binance USDⓈ-M order book updates file.
//
// This is the only test in the repository that validates a vendor's L2 field
// semantics against the vendor's own data. The specifications do not say what
// `sequence` or `metadata.firstUpdateId` mean, so they were established here.
func TestBookReader_RealSample(t *testing.T) {
	r, err := OpenBook(fixture, FeatureOrderBookUpdates, types.ExchangeBinance, "BTCUSDT")
	require.NoError(t, err)
	defer r.Close()

	events, err := r.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, events)

	// The fixture has 400 rows covering 203 distinct sequences: 197 changes
	// touched both sides and 6 touched only one, which is normal — a diff need
	// not move both books.
	assert.Equal(t, 203, len(events),
		"each change is stored as one row per side and must be paired back together")

	first := events[0]
	assert.Equal(t, marketdata.EventTypeBookUpdate, first.Type)
	assert.Equal(t, types.ExchangeBinance, first.Exchange)
	assert.Equal(t, "BTCUSDT", first.Symbol)

	// Both sides must be present on one event. A row pair decoded as two
	// separate events would each look like the other side vanishing.
	require.NotNil(t, first.Book)
	assert.NotEmpty(t, first.Book.Bids, "the bid side must be paired in")
	assert.NotEmpty(t, first.Book.Asks, "the ask side must be paired in")

	// Verified against the sample: sequence is Binance's "u".
	assert.Equal(t, uint64(9911346762831), first.Key.Seq)
	assert.Equal(t, int64(9911346762831), first.Book.LastUpdateId)

	// And metadata.firstUpdateId is Binance's "U", so PrevSeq is U-1: an
	// unbroken diff stream has U == previous u + 1.
	assert.Equal(t, uint64(9911346752134-1), first.PrevSeq,
		"firstUpdateId is what makes contiguity provable from the flat files")

	assert.Equal(t, int64(1771113600030), first.Time().UnixMilli())

	for i := 1; i < len(events); i++ {
		assert.LessOrEqual(t, events[i-1].Key.Compare(events[i].Key), 0,
			"the file must decode in order at index %d", i)
	}
}

// TestBookReader_PrevSeqMapsToFirstUpdateId establishes what firstUpdateId
// means, and records an important limitation of the public sample.
//
// Binance's rule for an unbroken diff stream is U == previous u + 1. That
// relation does hold in this data — exactly, on 13-digit numbers, which is not
// something that happens by accident — so the mapping of `sequence` to u and
// `metadata.firstUpdateId` to U is established.
//
// But it holds only rarely: across the full 136,669-event public sample the
// relation is satisfied 190 times, about 0.1%. The sample is decimated, not a
// complete capture, which is unsurprising for a free sample of a paid product.
// So this proves the field mapping and does NOT prove the feed is gap-free;
// whether the paid files are complete is listed as unverified in the parent
// package's documentation.
func TestBookReader_PrevSeqMapsToFirstUpdateId(t *testing.T) {
	r, err := OpenBook(fixture, FeatureOrderBookUpdates, types.ExchangeBinance, "BTCUSDT")
	require.NoError(t, err)
	defer r.Close()

	events, err := r.ReadAll()
	require.NoError(t, err)
	require.Greater(t, len(events), 2)

	var chained, broken int
	for i := 1; i < len(events); i++ {
		if events[i].PrevSeq == events[i-1].Key.Seq {
			chained++
		} else {
			broken++
		}
	}

	assert.Positive(t, chained,
		"U == previous u + 1 must hold at least once, or firstUpdateId is being misread")
	t.Logf("%d of %d consecutive events chain; %d show a gap (the sample is decimated)",
		chained, len(events)-1, broken)

	// The point of decoding the field: a gap becomes a reported error rather
	// than something no check can see. Over REST, where firstUpdateId is not
	// exposed, these would all be counted as unverifiable instead.
	book := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	book.Mode = marketdata.SequenceContiguous

	seed := events[0]
	seed.Type = marketdata.EventTypeBookSnapshot
	require.NoError(t, book.Apply(&seed))

	var gaps int
	for i := 1; i < len(events); i++ {
		if err := book.Apply(&events[i]); err != nil {
			require.ErrorIs(t, err, marketdata.ErrBookGap)
			gaps++

			// Re-seed so the walk continues past the hole.
			reseed := events[i]
			reseed.Type = marketdata.EventTypeBookSnapshot
			require.NoError(t, book.Apply(&reseed))
		}
	}

	assert.Zero(t, book.Unverified(),
		"with firstUpdateId decoded nothing is left unverifiable, unlike over REST")
	assert.Equal(t, broken, gaps, "every broken link must surface as a reported gap")
}

// TestBookReader_ZeroVolumeSurvives checks the removal signal reaches the
// consumer. The sample is a diff feed, so zero volumes are expected in it.
func TestBookReader_ZeroVolumeSurvives(t *testing.T) {
	r, err := OpenBook(fixture, FeatureOrderBookUpdates, types.ExchangeBinance, "BTCUSDT")
	require.NoError(t, err)
	defer r.Close()

	events, err := r.ReadAll()
	require.NoError(t, err)

	var zeros int
	for _, ev := range events {
		for _, side := range []types.PriceVolumeSlice{ev.Book.Bids, ev.Book.Asks} {
			for _, level := range side {
				if level.Volume.IsZero() {
					zeros++
				}
			}
		}
	}

	assert.Positive(t, zeros,
		"a diff feed must contain removals, and a zero volume is how they are expressed")
}

// TestBookReader_SnapshotFeature checks the feature selects the event type. The
// two products share a schema, so nothing in the file itself distinguishes them.
func TestBookReader_SnapshotFeature(t *testing.T) {
	r, err := OpenBook(fixture, FeatureOrderBookSnapshots, types.ExchangeBinance, "BTCUSDT")
	require.NoError(t, err)
	defer r.Close()

	ev, err := r.Read()
	require.NoError(t, err)

	assert.Equal(t, marketdata.EventTypeBookSnapshot, ev.Type)
	assert.Equal(t, marketdata.RankBookSnapshot, ev.Key.Rank)
}

// TestBookReader_FallsBackToFileNaming covers omitting the overrides: the rows
// carry the venue's own exchange and instrument.
func TestBookReader_FallsBackToFileNaming(t *testing.T) {
	r, err := OpenBook(fixture, FeatureOrderBookUpdates, "", "")
	require.NoError(t, err)
	defer r.Close()

	ev, err := r.Read()
	require.NoError(t, err)

	assert.Equal(t, types.ExchangeName("binance"), ev.Exchange)
	assert.Equal(t, "BTCUSDT", ev.Symbol)
}

// TestInspectSchema is the tool for a feature or venue with no decoder yet, and
// it documents the layout this package was written against.
func TestInspectSchema(t *testing.T) {
	schema, err := InspectSchema(fixture)
	require.NoError(t, err)

	for _, field := range []string{
		"isBid", "sequence", "firstUpdateId", "exchangeTimestamp", "data",
	} {
		assert.Contains(t, schema, field)
	}

	t.Logf("schema:\n%s", schema)
}

func TestOpenBook_MissingFile(t *testing.T) {
	_, err := OpenBook("testdata/does-not-exist.parquet", FeatureOrderBookUpdates, "", "")
	assert.Error(t, err)
}
