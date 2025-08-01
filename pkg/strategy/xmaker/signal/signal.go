package signal

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type KLineShapeSignal struct {
	FullBodyThreshold float64 `json:"fullBodyThreshold"`
}

type Config struct {
	Weight                   float64                         `json:"weight"`
	BollingerBandTrendSignal *BollingerBandTrendSignal       `json:"bollingerBandTrend,omitempty"`
	OrderBookBestPriceSignal *OrderBookBestPriceVolumeSignal `json:"orderBookBestPrice,omitempty"`
	DepthRatioSignal         *DepthRatioSignal               `json:"depthRatio,omitempty"`
	KLineShapeSignal         *KLineShapeSignal               `json:"klineShape,omitempty"`
	TradeVolumeWindowSignal  *TradeVolumeWindowSignal        `json:"tradeVolumeWindow,omitempty"`
}

func (c *Config) Get() types.SignalProvider {
	if c.OrderBookBestPriceSignal != nil {
		return c.OrderBookBestPriceSignal
	} else if c.DepthRatioSignal != nil {
		return c.DepthRatioSignal
	} else if c.BollingerBandTrendSignal != nil {
		return c.BollingerBandTrendSignal
	} else if c.TradeVolumeWindowSignal != nil {
		return c.TradeVolumeWindowSignal
	}

	panic(fmt.Errorf("no valid signal provider found, please check your config"))
}
