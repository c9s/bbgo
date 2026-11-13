// Package cloudsync reads AmberData's CloudSync flat files, the Parquet bulk
// delivery of the same data the REST API serves.
//
// The layout is materially different from the REST responses, so this is a
// separate decoder rather than a variation on the same one:
//
//   - REST returns one record per book change, with parallel ask[] and bid[]
//     arrays. Parquet emits one row per SIDE, tagged with isBid, so the two
//     halves of one change have to be paired back together.
//   - REST exposes only `sequence`. Parquet also carries
//     metadata.firstUpdateId, which is Binance's "U" — the first update id
//     covered by the event. That is what makes contiguity provable here and not
//     over REST.
//   - Parquet stores prices and volumes as float64 rather than as decimal
//     strings, so a value is only as exact as a float64 can be.
//
// Files are named
// market/{spot|futures|options}/{feature}/[{granularity}/]{date}.{exchange}.{instrument}.00.parquet.
// Obtaining them is left to the caller: the bulk bucket is Requester Pays, and
// pulling an AWS SDK in to fetch a file the user can fetch with one aws-cli
// command is not worth the dependency. Sample files are public and need no
// credentials, which is what this package was verified against.
package cloudsync

// metadataRow is the nested metadata group.
type metadataRow struct {
	// FirstUpdateID is Binance's "U": the first update id this event covers.
	// Verified against a real sample: the next event's FirstUpdateID equals the
	// previous event's Sequence plus one when nothing was lost.
	FirstUpdateID int64 `parquet:"firstUpdateId,optional"`

	// Version and LastID are present in the schema but empty in the Binance
	// samples inspected, so nothing is derived from them.
	Version int64  `parquet:"version,optional"`
	LastID  int64  `parquet:"lastId,optional"`
	Mrid    string `parquet:"mrid,optional"`
	ID      string `parquet:"id,optional"`
}

// bookRow is one row of an order-book-updates or order-book-snapshots file.
//
// One logical book change produces up to two rows, one per side, sharing a
// timestamp and sequence.
type bookRow struct {
	Exchange   string `parquet:"exchange,optional"`
	Instrument string `parquet:"instrument,optional"`

	ExchangeTimestamp            int64 `parquet:"exchangeTimestamp,optional"`
	ExchangeTimestampNanoseconds int64 `parquet:"exchangeTimestampNanoseconds,optional"`

	// IsBid selects which side this row's levels belong to.
	IsBid bool `parquet:"isBid,optional"`

	// ReceivedTimestamp is AmberData's ingestion time. It is recorded but not
	// used for ordering: only the venue's own time is meaningful under
	// simulated time.
	ReceivedTimestamp            int64 `parquet:"receivedTimestamp,optional"`
	ReceivedTimestampNanoseconds int64 `parquet:"receivedTimestampNanoseconds,optional"`

	Timestamp int64 `parquet:"timestamp,optional"`

	Metadata metadataRow `parquet:"metadata,optional"`

	// Sequence is Binance's "u": the last update id this event covers.
	Sequence int64 `parquet:"sequence,optional"`

	// Data is a list of [price, volume] pairs. A volume of zero means the level
	// was removed, the same convention as the REST events and as Binance itself.
	Data [][]float64 `parquet:"data,list,optional"`

	Status string `parquet:"status,optional"`
}

// tradeRow is one row of a trades file.
type tradeRow struct {
	Exchange   string `parquet:"exchange,optional"`
	Instrument string `parquet:"instrument,optional"`
	Pair       string `parquet:"pair,optional"`

	ExchangeTimestamp            int64 `parquet:"exchangeTimestamp,optional"`
	ExchangeTimestampNanoseconds int64 `parquet:"exchangeTimestampNanoseconds,optional"`
	ReceivedTimestamp            int64 `parquet:"receivedTimestamp,optional"`
	ReceivedTimestampNanoseconds int64 `parquet:"receivedTimestampNanoseconds,optional"`

	TradeID string `parquet:"tradeId,optional"`

	IsBuySide     bool `parquet:"isBuySide,optional"`
	IsLiquidation bool `parquet:"isLiquidation,optional"`

	Price       float64 `parquet:"price,optional"`
	Volume      float64 `parquet:"volume,optional"`
	QuoteVolume float64 `parquet:"quoteVolume,optional"`

	// Size and QuoteSize are the spot files' names for Volume and QuoteVolume.
	Size      float64 `parquet:"size,optional"`
	QuoteSize float64 `parquet:"quoteSize,optional"`
}

// eventTimeNano folds the sub-millisecond field into the millisecond timestamp.
func eventTimeNano(millis, nanos int64) int64 {
	return millis*1_000_000 + nanos
}
