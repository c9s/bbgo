package xmaker

import (
	"context"
	"fmt"
)

type SignalNumber float64

const (
	SignalNumberMaxLong  = 2.0
	SignalNumberMaxShort = -2.0
)

type SignalProvider interface {
	CalculateSignal(ctx context.Context) (float64, error)
}

type KLineShapeSignal struct {
	FullBodyThreshold float64 `json:"fullBodyThreshold"`
}

type SignalConfig struct {
	Weight                   float64                         `json:"weight"`
	BollingerBandTrendSignal *BollingerBandTrendSignal       `json:"bollingerBandTrend,omitempty"`
	OrderBookBestPriceSignal *OrderBookBestPriceVolumeSignal `json:"orderBookBestPrice,omitempty"`
	DepthRatioSignal         *DepthRatioSignal               `json:"depthRatio,omitempty"`
	KLineShapeSignal         *KLineShapeSignal               `json:"klineShape,omitempty"`
	TradeVolumeWindowSignal  *TradeVolumeWindowSignal        `json:"tradeVolumeWindow,omitempty"`
}

func (c *SignalConfig) Get() SignalProvider {
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
