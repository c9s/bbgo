package max

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PublicService struct {
	client *RestClient
}

type Market struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Status             string           `json:"market_status"` // active
	BaseUnit           string           `json:"base_unit"`
	BaseUnitPrecision  int              `json:"base_unit_precision"`
	QuoteUnit          string           `json:"quote_unit"`
	QuoteUnitPrecision int              `json:"quote_unit_precision"`
	MinBaseAmount      fixedpoint.Value `json:"min_base_amount"`
	MinQuoteAmount     fixedpoint.Value `json:"min_quote_amount"`
	SupportMargin      bool             `json:"m_wallet_supported"`
}

type Ticker struct {
	Time time.Time

	At          int64            `json:"at"`
	Buy         fixedpoint.Value `json:"buy"`
	Sell        fixedpoint.Value `json:"sell"`
	Open        fixedpoint.Value `json:"open"`
	High        fixedpoint.Value `json:"high"`
	Low         fixedpoint.Value `json:"low"`
	Last        fixedpoint.Value `json:"last"`
	Volume      fixedpoint.Value `json:"vol"`
	VolumeInBTC fixedpoint.Value `json:"vol_in_btc"`
}

func (s *PublicService) Timestamp(ctx context.Context) (int64, error) {
	req := s.client.NewGetTimestampRequest()
	ts, err := req.Do(ctx)
	if err != nil || ts == nil {
		return 0, nil
	}

	return int64(*ts), nil
}

func (s *PublicService) Markets(ctx context.Context) ([]Market, error) {
	req := s.client.NewGetMarketsRequest()
	return req.Do(ctx)
}

func (s *PublicService) Tickers(ctx context.Context) (TickerMap, error) {
	req := s.client.NewGetTickersRequest()
	return req.Do(ctx)
}

func (s *PublicService) Ticker(market string) (*Ticker, error) {
	req := s.client.NewGetTickerRequest()
	req.Market(market)
	return req.Do(context.Background())
}

type Interval int64

func ParseInterval(a string) (Interval, error) {
	switch strings.ToLower(a) {

	case "1m":
		return 1, nil

	case "5m":
		return 5, nil

	case "15m":
		return 15, nil

	case "30m":
		return 30, nil

	case "1h":
		return 60, nil

	case "2h":
		return 60 * 2, nil

	case "3h":
		return 60 * 3, nil

	case "4h":
		return 60 * 4, nil

	case "6h":
		return 60 * 6, nil

	case "8h":
		return 60 * 8, nil

	case "12h":
		return 60 * 12, nil

	case "1d":
		return 60 * 24, nil

	case "3d":
		return 60 * 24 * 3, nil

	case "1w":
		return 60 * 24 * 7, nil

	}

	return 0, fmt.Errorf("incorrect resolution: %q", a)
}

type KLine struct {
	Symbol                 string
	Interval               string
	StartTime, EndTime     time.Time
	Open, High, Low, Close fixedpoint.Value
	Volume                 fixedpoint.Value
	Closed                 bool
}

func (k KLine) KLine() types.KLine {
	return types.KLine{
		Exchange:  types.ExchangeMax,
		Symbol:    strings.ToUpper(k.Symbol), // global symbol
		Interval:  types.Interval(k.Interval),
		StartTime: types.Time(k.StartTime),
		EndTime:   types.Time(k.EndTime),
		Open:      k.Open,
		Close:     k.Close,
		High:      k.High,
		Low:       k.Low,
		Volume:    k.Volume,
		// QuoteVolume:    util.MustParseFloat(k.QuoteAssetVolume),
		// LastTradeID:    0,
		// NumberOfTrades: k.TradeNum,
		Closed: k.Closed,
	}
}

func (s *PublicService) KLines(symbol string, resolution string, start time.Time, limit int) ([]KLine, error) {
	interval, err := ParseInterval(resolution)
	if err != nil {
		return nil, err
	}

	req := s.client.NewGetKLinesRequest()
	req.Market(symbol).Period(int(interval)).Timestamp(start).Limit(limit)
	data, err := req.Do(context.Background())
	if err != nil {
		return nil, err
	}

	var kLines []KLine
	for _, slice := range data {
		ts := int64(slice[0])
		startTime := time.Unix(ts, 0)
		endTime := startTime.Add(time.Duration(interval)*time.Minute - time.Millisecond)
		isClosed := time.Now().Before(endTime)
		kLines = append(kLines, KLine{
			Symbol:    symbol,
			Interval:  resolution,
			StartTime: startTime,
			EndTime:   endTime,
			Open:      fixedpoint.NewFromFloat(slice[1]),
			High:      fixedpoint.NewFromFloat(slice[2]),
			Low:       fixedpoint.NewFromFloat(slice[3]),
			Close:     fixedpoint.NewFromFloat(slice[4]),
			Volume:    fixedpoint.NewFromFloat(slice[5]),
			Closed:    isClosed,
		})
	}
	return kLines, nil
	// return parseKLines(resp.Body, symbol, resolution, interval)
}
