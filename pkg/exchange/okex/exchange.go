package okex

import (
	"context"
	"math"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

//go:generate sh -c "echo \"package okex\nvar symbolMap = map[string]string{\n\" $(curl -s -L 'https://okex.com/api/v5/public/instruments?instType=SPOT' | jq -r '.data[] | \"\\(.instId | sub(\"-\" ; \"\") | tojson ): \\( .instId | tojson),\n\"') \"\n}\" > symbols.go"

// OKB is the platform currency of OKEx, pre-allocate static string here
const OKB = "OKB"

var log = logrus.WithFields(logrus.Fields{
	"exchange": "okex",
})

type Exchange struct {
	key, secret, passphrase string

	client *okexapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := okexapi.NewClient()
	client.Auth(key, secret, passphrase)

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeOKEx
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	instruments, err := e.client.PublicDataService.NewGetInstrumentsRequest().
		InstrumentType(okexapi.InstrumentTypeSpot).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, instrument := range instruments {
		symbol := toGlobalSymbol(instrument.InstrumentID)
		market := types.Market{
			Symbol:      symbol,
			LocalSymbol: instrument.InstrumentID,

			QuoteCurrency: instrument.QuoteCurrency,
			BaseCurrency:  instrument.BaseCurrency,

			// convert tick size OKEx to precision
			PricePrecision:  int(-math.Log10(instrument.TickSize.Float64())),
			VolumePrecision: int(-math.Log10(instrument.LotSize.Float64())),

			// TickSize: OKEx's price tick, for BTC-USDT it's "0.1"
			TickSize: instrument.TickSize.Float64(),

			// Quantity step size, for BTC-USDT, it's "0.00000001"
			StepSize: instrument.LotSize.Float64(),

			// for BTC-USDT, it's "0.00001"
			MinQuantity: instrument.MinSize.Float64(),

			// OKEx does not offer minimal notional, use 1 USD here.
			MinNotional: 1.0,
			MinAmount:   1.0,
		}
		markets[symbol] = market
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	return nil, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol string) (*types.Ticker, error) {
	return nil, nil
}

func (e *Exchange) PlatformFeeCurrency() string {
	return OKB
}
