package bollmaker

import (
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type DynamicSpreadSettings struct {
	Enabled bool `json:"enabled"`

	types.IntervalWindow

	// AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`

	DynamicAskSpread *indicator.SMA
	DynamicBidSpread *indicator.SMA
}

// Update dynamic spreads
func (ds *DynamicSpreadSettings) Update(kline types.KLine) {
	if !ds.Enabled {
		return
	}

	ampl := (kline.GetHigh().Float64() - kline.GetLow().Float64()) / kline.GetOpen().Float64()

	switch kline.Direction() {
	case types.DirectionUp:
		ds.DynamicAskSpread.Update(ampl)
		ds.DynamicBidSpread.Update(0)
	case types.DirectionDown:
		ds.DynamicBidSpread.Update(ampl)
		ds.DynamicAskSpread.Update(0)
	default:
		ds.DynamicAskSpread.Update(0)
		ds.DynamicBidSpread.Update(0)
	}
}

// Initialize dynamic spreads and preload SMAs
func (ds *DynamicSpreadSettings) Initialize(symbol string, session *bbgo.ExchangeSession) {
	ds.DynamicBidSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}
	ds.DynamicAskSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}

	kLineStore, _ := session.MarketDataStore(symbol)
	if klines, ok := kLineStore.KLinesOfInterval(ds.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			ds.Update((*klines)[i])
		}
	}
}

// GetAskSpread returns current ask spread
func (ds *DynamicSpreadSettings) GetAskSpread() (askSpread float64, err error) {
	if !ds.Enabled {
		return 0, errors.New("dynamic spread is not enabled")
	}

	if ds.AskSpreadScale != nil && ds.DynamicAskSpread.Length() >= ds.Window {
		askSpread, err = ds.AskSpreadScale.Scale(ds.DynamicAskSpread.Last())
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicAskSpread")
			return 0, err
		}

		return askSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}

// GetBidSpread returns current dynamic bid spread
func (ds *DynamicSpreadSettings) GetBidSpread() (bidSpread float64, err error) {
	if !ds.Enabled {
		return 0, errors.New("dynamic spread is not enabled")
	}

	if ds.BidSpreadScale != nil && ds.DynamicBidSpread.Length() >= ds.Window {
		bidSpread, err = ds.BidSpreadScale.Scale(ds.DynamicBidSpread.Last())
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicBidSpread")
			return 0, err
		}

		return bidSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}
