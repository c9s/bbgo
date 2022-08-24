package indicator

import "github.com/c9s/bbgo/pkg/types"

type KLineValueMapper func(k types.KLine) float64

func KLineOpenPriceMapper(k types.KLine) float64 {
	return k.Open.Float64()
}

func KLineClosePriceMapper(k types.KLine) float64 {
	return k.Close.Float64()
}

func KLineTypicalPriceMapper(k types.KLine) float64 {
	return (k.High.Float64() + k.Low.Float64() + k.Close.Float64()) / 3.
}

func KLinePriceVolumeMapper(k types.KLine) float64 {
	return k.Close.Mul(k.Volume).Float64()
}

func KLineVolumeMapper(k types.KLine) float64 {
	return k.Volume.Float64()
}

func MapKLinePrice(kLines []types.KLine, f KLineValueMapper) (prices []float64) {
	for _, k := range kLines {
		prices = append(prices, f(k))
	}

	return prices
}
