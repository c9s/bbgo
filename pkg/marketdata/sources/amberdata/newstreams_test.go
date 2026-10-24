package amberdata

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
	"github.com/c9s/bbgo/pkg/types"
)

func decodeRecord[T any](t *testing.T, raw string) T {
	t.Helper()

	var out T
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

// TestSource_Liquidations is the first producer of EventTypeLiquidation anywhere
// in the layer: no public archive publishes liquidations, so until this endpoint
// the event type had a definition and no source.
func TestSource_Liquidations(t *testing.T) {
	liq := decodeRecord[amberdataapi.Liquidation](t, `{
		"instrument":"BTCUSDT","exchange":"binance",
		"exchangeTimestamp":1717517743430,"exchangeTimestampNanoseconds":0,
		"action":null,"orderId":null,"side":"BUY","status":"FILLED",
		"timeInForce":"IOC","type":"LIMIT","price":70913.39,"volume":0.002,
		"unit":"base_asset","positionType":"SHORT"}`)

	src := newSource(t, &fakeClient{liquidations: []amberdataapi.Liquidation{liq}})

	assert.Contains(t, src.Capabilities().Channels, types.LiquidationOrderChannel)

	cur, err := src.Open(context.Background(), baseRequest(types.Subscription{
		Symbol: "BTCUSDT", Channel: types.LiquidationOrderChannel,
	}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 1)

	ev := got[0]
	assert.Equal(t, marketdata.EventTypeLiquidation, ev.Type)
	assert.Equal(t, marketdata.RankLiquidation, ev.Key.Rank)

	require.NotNil(t, ev.Liquidation)
	assert.Equal(t, "70913.39", ev.Liquidation.Price.String())
	assert.Equal(t, "0.002", ev.Liquidation.Quantity.String())
	assert.Equal(t, types.SideTypeBuy, ev.Liquidation.Side,
		"side is the liquidating order's side: a short is closed by buying back")

	// positionType and unit have no home on types.LiquidationInfo, and unit in
	// particular matters because some venues report contracts, so the raw record
	// is carried alongside.
	raw, ok := ev.Raw.(amberdataapi.Liquidation)
	require.True(t, ok, "the raw record must be reachable for the fields types.LiquidationInfo lacks")
	assert.Equal(t, "SHORT", raw.PositionType)
	assert.Equal(t, "base_asset", raw.Unit)
}

// TestSource_FundingRates maps onto the mark price channel, which is where bbgo's
// streams carry the funding rate.
func TestSource_FundingRates(t *testing.T) {
	settled := decodeRecord[amberdataapi.FundingRate](t, `{
		"instrument":"BTCUSD_PERP","exchange":"binance",
		"exchangeTimestamp":1717545600000,"exchangeTimestampNanoseconds":0,
		"fundingInterval":null,"fundingRate":0.0001,"nextFundingRate":null,
		"nextFundingTime":null,"isActualFundingRate":true}`)

	predicted := decodeRecord[amberdataapi.FundingRate](t, `{
		"instrument":"BTCUSD_PERP","exchange":"binance",
		"exchangeTimestamp":1717545601000,"exchangeTimestampNanoseconds":0,
		"fundingInterval":null,"fundingRate":0.0002,"nextFundingRate":null,
		"nextFundingTime":null,"isActualFundingRate":false}`)

	src := newSource(t, &fakeClient{
		fundingRates: []amberdataapi.FundingRate{settled, predicted},
	})

	cur, err := src.Open(context.Background(), marketdata.Request{
		Since: baseRequest().Since, Until: baseRequest().Until,
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarkPriceChannel},
		},
	})
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 2)

	assert.Equal(t, marketdata.EventTypeMarkPrice, got[0].Type)
	require.NotNil(t, got[0].MarkPrice)
	assert.Equal(t, "0.0001", got[0].MarkPrice.LastFundingRate.String())
	assert.False(t, got[0].Flags.Has(marketdata.FlagSynthetic),
		"a settled rate is observed, not derived")

	assert.True(t, got[1].Flags.Has(marketdata.FlagSynthetic),
		"a predicted rate must be marked, since it is not what a position paid")
}

func TestSource_Tickers(t *testing.T) {
	ticker := decodeRecord[amberdataapi.Ticker](t, `{
		"instrument":"BTCUSDT","exchange":"binance",
		"exchangeTimestamp":1717518473606,"exchangeTimestampNanoseconds":0,
		"bid":70836.3,"ask":70836.4,"mid":70836.35,"last":null,
		"bidVolume":2.016,"askVolume":6.463}`)

	src := newSource(t, &fakeClient{tickers: []amberdataapi.Ticker{ticker}})

	cur, err := src.Open(context.Background(), baseRequest(types.Subscription{
		Symbol: "BTCUSDT", Channel: types.BookTickerChannel,
	}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 1)

	assert.Equal(t, marketdata.EventTypeBookTicker, got[0].Type)
	require.NotNil(t, got[0].BookTicker)
	assert.Equal(t, "70836.3", got[0].BookTicker.Buy.String())
	assert.Equal(t, "6.463", got[0].BookTicker.SellSize.String())

	// The API reports one timestamp, so there is no publication latency to
	// record and both are set to it rather than one being invented.
	assert.Equal(t, got[0].BookTicker.TransactionTime, got[0].BookTicker.EventTime)
}

// TestSource_MergesEverything is the point of adding these endpoints: trades,
// book, liquidations and funding all come out of one source in one ordered
// stream.
func TestSource_MergesEverything(t *testing.T) {
	src := newSource(t, &fakeClient{
		tradePages: [][]amberdataapi.Trade{{apiTrade(1_717_518_000_100, "1", true)}},
		snapPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_000, "5000",
				`{"price":100,"volume":1,"numOrders":null}`,
				`{"price":101,"volume":1,"numOrders":null}`),
		}},
		bookPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_300, "5001", `{"price":100,"volume":2,"numOrders":null}`, ``),
		}},
		liquidations: []amberdataapi.Liquidation{decodeRecord[amberdataapi.Liquidation](t, `{
			"instrument":"BTCUSDT","exchange":"binance",
			"exchangeTimestamp":1717518000200,"exchangeTimestampNanoseconds":0,
			"side":"SELL","status":"FILLED","timeInForce":"IOC","type":"LIMIT",
			"price":100.5,"volume":0.5,"unit":"base_asset","positionType":"LONG"}`)},
	})

	cur, err := src.Open(context.Background(), baseRequest(
		types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		types.Subscription{Symbol: "BTCUSDT", Channel: types.BookChannel},
		types.Subscription{Symbol: "BTCUSDT", Channel: types.LiquidationOrderChannel},
	))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 4)

	var prev marketdata.OrderKey
	for _, ev := range got {
		assert.LessOrEqual(t, prev.Compare(ev.Key), 0,
			"four endpoints merged into one source must still come out ordered")
		prev = ev.Key
	}

	assert.Equal(t, []marketdata.EventType{
		marketdata.EventTypeBookSnapshot, // t=...000
		marketdata.EventTypeTrade,        // t=...100
		marketdata.EventTypeLiquidation,  // t=...200
		marketdata.EventTypeBookUpdate,   // t=...300
	}, []marketdata.EventType{got[0].Type, got[1].Type, got[2].Type, got[3].Type})
}

func TestSource_OpenInterestAndRatioBecomeMetrics(t *testing.T) {
	oi := decodeRecord[amberdataapi.OpenInterest](t, `{
		"instrument":"BTCUSDT","exchange":"binance",
		"exchangeTimestamp":1717518065771,"exchangeTimestampNanoseconds":0,
		"value":16862174,"type":"PERPETUAL"}`)

	ratio := decodeRecord[amberdataapi.LongShortRatio](t, `{
		"instrument":"BTCUSDT","exchange":"binance",
		"exchangeTimestamp":1717518065771,"exchangeTimestampNanoseconds":0,
		"period":1,"longAccount":0.519,"shortAccount":0.481,"ratio":1.079}`)

	// These two have no stream channel of their own, so they are reached through
	// the converters directly rather than through a subscription.
	oiEvent := openInterestToEvent(oi, types.ExchangeBinance, "BTCUSDT")
	assert.Equal(t, marketdata.EventTypeMetrics, oiEvent.Type)
	assert.Equal(t, "16862174",
		oiEvent.Metrics.Values[marketdata.MetricSumOpenInterest].String())

	ratioEvent := longShortRatioToEvent(ratio, types.ExchangeBinance, "BTCUSDT")
	assert.Equal(t, marketdata.EventTypeMetrics, ratioEvent.Type)
	assert.Equal(t, "1.079",
		ratioEvent.Metrics.Values[marketdata.MetricCountLongShortRatio].String())
	assert.Equal(t, "0.519", ratioEvent.Metrics.Values[MetricLongAccount].String())
	assert.Equal(t, "0.481", ratioEvent.Metrics.Values[MetricShortAccount].String())
}
