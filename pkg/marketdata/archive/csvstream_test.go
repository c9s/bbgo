package archive

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseEpochNano_RealBinanceValues uses timestamps taken verbatim from
// data.binance.vision archives. The spot values are the reason this function
// exists: Binance moved spot to microseconds while futures stayed on
// milliseconds, with no change to the column name.
func TestParseEpochNano_RealBinanceValues(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantUnit EpochUnit
		wantUTC  string
	}{
		{
			name:     "futures um aggTrades 2026-09-15, milliseconds",
			in:       "1789430400003",
			wantUnit: EpochMilliseconds,
			wantUTC:  "2026-09-15T00:00:00.003Z",
		},
		{
			name:     "spot aggTrades 2026-09-15, microseconds",
			in:       "1789430400017025",
			wantUnit: EpochMicroseconds,
			wantUTC:  "2026-09-15T00:00:00.017025Z",
		},
		{
			name:     "spot aggTrades 2023-11-17, still milliseconds",
			in:       "1700179200000",
			wantUnit: EpochMilliseconds,
			wantUTC:  "2023-11-17T00:00:00Z",
		},
		{
			name:     "spot klines 2026-09-15 open time, microseconds",
			in:       "1789430400000000",
			wantUnit: EpochMicroseconds,
			wantUTC:  "2026-09-15T00:00:00Z",
		},
		{
			name:     "spot klines close time, microseconds",
			in:       "1789430459999999",
			wantUnit: EpochMicroseconds,
			wantUTC:  "2026-09-15T00:00:59.999999Z",
		},
		{
			name:     "futures um klines close time, milliseconds",
			in:       "1789430459999",
			wantUnit: EpochMilliseconds,
			wantUTC:  "2026-09-15T00:00:59.999Z",
		},
		{
			name:     "legacy scientific notation from older kline files",
			in:       "1.70027E+12",
			wantUnit: EpochMilliseconds,
			wantUTC:  "2023-11-18T01:13:20Z",
		},
		{
			name:     "seconds",
			in:       "1700179200",
			wantUnit: EpochSeconds,
			wantUTC:  "2023-11-17T00:00:00Z",
		},
		{
			name:     "nanoseconds",
			in:       "1789430400017025000",
			wantUnit: EpochNanoseconds,
			wantUTC:  "2026-09-15T00:00:00.017025Z",
		},
		{
			name:     "surrounding whitespace is tolerated",
			in:       "  1789430400003  ",
			wantUnit: EpochMilliseconds,
			wantUTC:  "2026-09-15T00:00:00.003Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, unit, err := ParseEpochNano(tt.in)
			require.NoError(t, err)

			assert.Equal(t, tt.wantUnit, unit, "unit")
			assert.Equal(t, tt.wantUTC,
				time.Unix(0, ns).UTC().Format("2006-01-02T15:04:05.999999999Z"))
		})
	}
}

// TestParseEpochNano_MillisecondsAndMicrosecondsAgree is the property that
// makes magnitude detection safe: the same instant expressed in either unit
// must decode to the same time.
func TestParseEpochNano_MillisecondsAndMicrosecondsAgree(t *testing.T) {
	msNs, _, err := ParseEpochNano("1789430400000")
	require.NoError(t, err)

	usNs, _, err := ParseEpochNano("1789430400000000")
	require.NoError(t, err)

	assert.Equal(t, msNs, usNs)
}

func TestParseEpochNano_Errors(t *testing.T) {
	for _, in := range []string{"", "   ", "not-a-number", "-1700179200000", "NaN", "Inf"} {
		t.Run(in, func(t *testing.T) {
			_, _, err := ParseEpochNano(in)
			assert.Error(t, err)
		})
	}
}

func TestParseDateTime(t *testing.T) {
	// verbatim from a futures um metrics archive
	got, err := ParseDateTime("2026-09-15 00:00:00")
	require.NoError(t, err)

	assert.Equal(t, "2026-09-15T00:00:00Z", got.Format(time.RFC3339))
	assert.Equal(t, time.UTC, got.Location())
}

// TestReader_HeaderDetection covers both layouts Binance ships within one
// dataset family: futures archives carry a header, spot archives do not.
func TestReader_HeaderDetection(t *testing.T) {
	t.Run("with header", func(t *testing.T) {
		// verbatim shape of futures/um/daily/aggTrades
		in := "agg_trade_id,price,quantity,first_trade_id,last_trade_id,transact_time,is_buyer_maker\n" +
			"3449899747,78153.0,0.001,8078264332,8078264332,1789430400003,false\n"

		r := NewReader(strings.NewReader(in))

		rec, err := r.Read()
		require.NoError(t, err)
		assert.Equal(t, "3449899747", rec[0], "the header row must not be returned as data")

		require.NotNil(t, r.Columns())
		idx, ok := r.Column("is_buyer_maker")
		require.True(t, ok)
		assert.Equal(t, 6, idx)

		_, err = r.Read()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("without header", func(t *testing.T) {
		// verbatim shape of spot/daily/aggTrades, with microsecond time
		in := "4063759318,78189.20000000,0.00009000,6681954163,6681954164,1789430400017025,True,True\n"

		r := NewReader(strings.NewReader(in))

		rec, err := r.Read()
		require.NoError(t, err)
		assert.Equal(t, "4063759318", rec[0], "the first row is data and must not be eaten")
		assert.Nil(t, r.Columns())
		assert.Nil(t, r.Header())
	})
}

func TestReader_RequireColumns(t *testing.T) {
	in := "id,price,qty,quote_qty,time,is_buyer_maker\n" +
		"8078264332,78153.0,0.001,78.153,1789430400003,false\n"

	r := NewReader(strings.NewReader(in))
	_, err := r.Read()
	require.NoError(t, err)

	require.NoError(t, r.RequireColumns("id", "price", "time"))

	err = r.RequireColumns("id", "settlement_price")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settlement_price",
		"an inserted or renamed column must fail loudly, not shift fields silently")
}

// TestReader_RequireColumnsHeaderless documents that a headerless file cannot
// be validated, so decoders fall back to fixed positions there.
func TestReader_RequireColumnsHeaderless(t *testing.T) {
	r := NewReader(strings.NewReader("1,2,3\n"))
	_, err := r.Read()
	require.NoError(t, err)

	assert.NoError(t, r.RequireColumns("anything"))
}

func TestReader_LineNumbers(t *testing.T) {
	in := "id,price\n1,100\n2,101\n"
	r := NewReader(strings.NewReader(in))

	_, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, 2, r.Line(), "line numbers count the header row")

	_, err = r.Read()
	require.NoError(t, err)
	assert.Equal(t, 3, r.Line())
}

func TestReader_CRLF(t *testing.T) {
	in := "id,price\r\n1,100\r\n"
	r := NewReader(strings.NewReader(in))

	rec, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "100"}, rec)
}

// TestLooksNumeric documents the header/data discriminator, including the
// mojibake case: an OKEx archive's header is GBK-encoded and renders as
// unreadable bytes, which are still not numeric and so still detected.
func TestLooksNumeric(t *testing.T) {
	assert.True(t, looksNumeric("123"))
	assert.True(t, looksNumeric("1.70027E+12"))
	assert.True(t, looksNumeric(" 1789430400003 "))

	assert.False(t, looksNumeric("agg_trade_id"))
	assert.False(t, looksNumeric(""))
	assert.False(t, looksNumeric("\xbd\xbb\xd2\xd7id")) // GBK mojibake
}
