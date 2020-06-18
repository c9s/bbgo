package bbgo

import (
	"github.com/adshao/go-binance"
	"math"
)

// this is for BTC
const MinQuantity = 0.00000100

// https://www.desmos.com/calculator/ircjhtccbn
func BuyVolumeModifier(price float64) float64 {
	targetPrice := 7500.0 // we will get 1 at price 7500, and more below 7500
	flatness := 1000.0    // higher number buys more in the middle section. higher number gets more flat line, reduced to 0 at price 2000 * 10
	return math.Min(2, math.Exp(-(price-targetPrice)/flatness))
}

func SellVolumeModifier(price float64) float64 {
	// \exp\left(\frac{x-10000}{500}\right)
	targetPrice := 10500.0 // target to sell most x1 at 10000.0
	flatness := 500.0      // higher number sells more in the middle section, lower number sells fewer in the middle section.
	return math.Min(2, math.Exp((price-targetPrice)/flatness))
}

func VolumeByPriceChange(market Market, currentPrice float64, change float64, side binance.SideType) float64 {
	volume := BaseVolumeByPriceChange(change)

	if side == binance.SideTypeSell {
		volume *= SellVolumeModifier(currentPrice)
	} else {
		volume *= volume*BuyVolumeModifier(currentPrice)
	}

	// at least the minimal quantity
	volume = math.Max(market.MinQuantity, volume)

	// modify volume for the min amount
	amount := currentPrice * volume
	if amount < market.MinAmount {
		ratio := market.MinAmount / amount
		volume *= ratio
	}

	volume = math.Trunc(volume * math.Pow10(market.VolumePrecision)) / math.Pow10(market.VolumePrecision)
	return volume
}

func BaseVolumeByPriceChange(change float64) float64 {
	return 0.12 * math.Exp((math.Abs(change)-3100.0)/1600.0)
	// 0.116*math.Exp(math.Abs(change)/2400) - 0.1
}

