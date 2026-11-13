package binance

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestIsFuturesPublicChannel(t *testing.T) {
	assert.True(t, isFuturesPublicChannel(types.BookChannel))
	assert.True(t, isFuturesPublicChannel(types.BookTickerChannel))
	assert.False(t, isFuturesPublicChannel(types.KLineChannel))
	assert.False(t, isFuturesPublicChannel(types.AggTradeChannel))
	assert.False(t, isFuturesPublicChannel(types.MarketTradeChannel))
	assert.False(t, isFuturesPublicChannel(types.MarkPriceChannel))
	assert.False(t, isFuturesPublicChannel(types.ForceOrderChannel))
}

func TestClassifyFuturesSubscriptions(t *testing.T) {
	subs := []types.Subscription{
		{Channel: types.BookChannel, Symbol: "BTCUSDT"},
		{Channel: types.BookTickerChannel, Symbol: "ETHUSDT"},
		{Channel: types.KLineChannel, Symbol: "BTCUSDT"},
		{Channel: types.AggTradeChannel, Symbol: "BTCUSDT"},
		{Channel: types.MarkPriceChannel, Symbol: "BTCUSDT"},
	}
	public, market := classifyFuturesSubscriptions(subs)
	assert.Len(t, public, 2)
	assert.Len(t, market, 3)
	assert.Equal(t, types.BookChannel, public[0].Channel)
	assert.Equal(t, types.BookTickerChannel, public[1].Channel)
	assert.Equal(t, types.KLineChannel, market[0].Channel)
}

func TestClassifyFuturesSubscriptions_OnlyPublic(t *testing.T) {
	subs := []types.Subscription{{Channel: types.BookChannel, Symbol: "BTCUSDT"}}
	public, market := classifyFuturesSubscriptions(subs)
	assert.Len(t, public, 1)
	assert.Empty(t, market)
}

func TestClassifyFuturesSubscriptions_OnlyMarket(t *testing.T) {
	subs := []types.Subscription{{Channel: types.KLineChannel, Symbol: "BTCUSDT"}}
	public, market := classifyFuturesSubscriptions(subs)
	assert.Empty(t, public)
	assert.Len(t, market, 1)
}

func TestGetFuturesPublicEndpointUrl(t *testing.T) {
	s := &Stream{exchange: &Exchange{}}
	testNet = false
	assert.Equal(t, FuturesPublicWebSocketURL+"/ws", s.getFuturesPublicEndpointUrl())
}

func TestGetFuturesMarketEndpointUrl(t *testing.T) {
	s := &Stream{exchange: &Exchange{}}
	testNet = false
	assert.Equal(t, FuturesMarketWebSocketURL+"/ws", s.getFuturesMarketEndpointUrl())
}

func TestGetFuturesPrivateEndpointUrl(t *testing.T) {
	s := &Stream{exchange: &Exchange{}}
	testNet = false
	url := s.getFuturesPrivateEndpointUrl("mykey123")
	assert.Equal(t, FuturesPrivateWebSocketURL+"/ws?listenKey=mykey123&events="+futuresPrivateStreamEvents, url)
}

func TestWriteSpecificSubscriptions_Empty(t *testing.T) {
	s := &Stream{exchange: &Exchange{}}
	// nil conn — should return nil without panicking when subs is empty
	err := s.writeSpecificSubscriptions(nil, nil)
	assert.NoError(t, err)
}

func TestHandleIndexPriceKLineEvent(t *testing.T) {
	s := &Stream{}

	// index price klines must not leak into the generic kline sink used by
	// kline-driven strategies/indicators — they are a different data source.
	var closedKLines []types.KLine
	s.OnKLineClosed(func(kline types.KLine) {
		closedKLines = append(closedKLines, kline)
	})

	var closedIndexPriceKLines []types.KLine
	s.OnIndexPriceKLineClosed(func(kline types.KLine) {
		closedIndexPriceKLines = append(closedIndexPriceKLines, kline)
	})

	e := &IndexPriceKLineEvent{
		Symbol: "BTCUSD",
		KLine: KLine{
			Symbol:    "0", // placeholder value sent by Binance, must not leak into the kline
			Closed:    true,
			StartTime: 1591267020000,
			EndTime:   1591267079999,
		},
	}
	s.handleIndexPriceKLineEvent(e)

	assert.Empty(t, closedKLines)
	if assert.Len(t, closedIndexPriceKLines, 1) {
		assert.Equal(t, "BTCUSD", closedIndexPriceKLines[0].Symbol)
	}

	// non-closed klines must emit the update sinks but not the closed ones
	closedIndexPriceKLines = nil
	var updatedIndexPriceKLines []types.KLine
	s.OnIndexPriceKLine(func(kline types.KLine) {
		updatedIndexPriceKLines = append(updatedIndexPriceKLines, kline)
	})

	e2 := &IndexPriceKLineEvent{Symbol: "BTCUSD", KLine: KLine{Closed: false}}
	s.handleIndexPriceKLineEvent(e2)

	assert.Empty(t, closedIndexPriceKLines)
	assert.Len(t, updatedIndexPriceKLines, 1)
}

func TestConvertSubscription_IndexPriceKLineChannel(t *testing.T) {
	sub := types.Subscription{
		Channel: types.IndexPriceKLineChannel,
		Symbol:  "BTCUSD",
		Options: types.SubscribeOptions{Interval: types.Interval1m},
	}
	assert.Equal(t, "btcusd@indexPriceKline_1m", convertSubscription(sub))
}

func TestGetPublicEndpointUrl_FuturesOnlyMarket(t *testing.T) {
	testNet = false
	ex := &Exchange{}
	ex.FuturesSettings.IsFutures = true
	s := &Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       ex,
	}
	s.Subscriptions = []types.Subscription{
		{Channel: types.KLineChannel, Symbol: "BTCUSDT"},
	}
	url := s.getPublicEndpointUrl()
	assert.Equal(t, FuturesMarketWebSocketURL+"/ws", url)
	assert.Nil(t, s.futuresAuxStream)
}

func TestGetPublicEndpointUrl_FuturesOnlyPublic(t *testing.T) {
	testNet = false
	ex := &Exchange{}
	ex.FuturesSettings.IsFutures = true
	s := &Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       ex,
	}
	s.Subscriptions = []types.Subscription{
		{Channel: types.BookChannel, Symbol: "BTCUSDT"},
	}
	url := s.getPublicEndpointUrl()
	assert.Equal(t, FuturesPublicWebSocketURL+"/ws", url)
	assert.Nil(t, s.futuresAuxStream)
}

func TestGetPublicEndpointUrl_FuturesMixed(t *testing.T) {
	testNet = false
	ex := &Exchange{}
	ex.FuturesSettings.IsFutures = true
	s := &Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       ex,
	}
	s.Subscriptions = []types.Subscription{
		{Channel: types.BookChannel, Symbol: "BTCUSDT"},
		{Channel: types.KLineChannel, Symbol: "BTCUSDT"},
	}
	url := s.getPublicEndpointUrl()
	assert.Equal(t, FuturesMarketWebSocketURL+"/ws", url)
	assert.NotNil(t, s.futuresAuxStream)
}

func TestGetUserDataStreamEndpointUrl_Futures(t *testing.T) {
	testNet = false
	ex := &Exchange{}
	ex.FuturesSettings.IsFutures = true
	s := &Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       ex,
	}
	url := s.getUserDataStreamEndpointUrl("listenkey123")
	assert.Equal(t, FuturesPrivateWebSocketURL+"/ws?listenKey=listenkey123&events="+futuresPrivateStreamEvents, url)
}
