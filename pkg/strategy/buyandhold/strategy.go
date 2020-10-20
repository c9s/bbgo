package buyandhold

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	bbgo.RegisterStrategy("buyandhold", &Strategy{})
}

type Strategy struct {
	Symbol            string  `json:"symbol"`
	Interval          string  `json:"interval"`
	BaseQuantity      float64 `json:"baseQuantity"`
	MaxAssetQuantity  float64 `json:"maxAssetQuantity"`
	MinDropPercentage float64 `json:"minDropPercentage"`
}

func LoadFile(filepath string) (*Strategy, error) {
	o, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var strategy Strategy
	err = json.Unmarshal(o, &strategy)
	return &strategy, err
}

func New(symbol string, interval string, baseQuantity float64) *Strategy {
	return &Strategy{
		Symbol:            symbol,
		Interval:          interval,
		BaseQuantity:      baseQuantity,
		MinDropPercentage: -0.08,
	}
}

func (s *Strategy) SetMinDropPercentage(p float64) *Strategy {
	s.MinDropPercentage = p
	return s
}

func (s *Strategy) SetMaxAssetQuantity(q float64) *Strategy {
	s.MaxAssetQuantity = q
	return s
}

func (s *Strategy) Run(ctx context.Context, orderExecutor types.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		changePercentage := kline.GetChange() / kline.Open

		// buy when price drops -8%
		if changePercentage < s.MinDropPercentage {
			market, ok := session.Market(s.Symbol)
			if !ok {
				return
			}

			baseBalance, ok := session.Account.Balance(market.BaseCurrency)
			if ok {
				// we hold too many
				if util.NotZero(s.MaxAssetQuantity) && baseBalance.Available > s.MaxAssetQuantity {
					return
				}
			}

			err := orderExecutor.SubmitOrder(ctx, types.SubmitOrder{
				Symbol:   kline.Symbol,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeMarket,
				Quantity: s.BaseQuantity * math.Abs(changePercentage),
			})
			if err != nil {
				log.WithError(err).Error("submit order error")
			}
		}
	})

	return nil
}
