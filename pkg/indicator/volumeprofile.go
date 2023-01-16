package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type trade struct {
	price     float64
	volume    float64 // +: buy, -: sell
	timestamp types.Time
}

// The Volume Profile is a technical analysis tool that is used to visualize the distribution of trading volume at different price levels
// in a security. It is typically plotted as a histogram or heatmap on the price chart, with the x-axis representing the price levels and
// the y-axis representing the trading volume. The Volume Profile can be used to identify areas of support and resistance, as well as
// potential entry and exit points for trades.
type VolumeProfile struct {
	types.IntervalWindow
	Delta    float64
	profile  map[float64]float64
	trades   []trade
	minPrice float64
	maxPrice float64
}

func (inc *VolumeProfile) Update(price, volume float64, timestamp types.Time) {
	inc.minPrice = math.Inf(1)
	inc.maxPrice = math.Inf(-1)
	if inc.profile == nil {
		inc.profile = make(map[float64]float64)
	}
	inc.profile[math.Round(price/inc.Delta)] += volume
	filter := timestamp.Time().Add(-time.Duration(inc.Window) * inc.Interval.Duration())
	inc.trades = append(inc.trades, trade{
		price:     price,
		volume:    volume,
		timestamp: timestamp,
	})
	var i int
	for i = 0; i < len(inc.trades); i++ {
		td := inc.trades[i]
		if td.timestamp.After(filter) {
			inc.trades = inc.trades[i:len(inc.trades)]
			break
		}
		inc.profile[math.Round(td.price/inc.Delta)] -= td.volume
	}

	for i = 0; i < len(inc.trades); i++ {
		k := math.Round(inc.trades[i].price / inc.Delta)
		if k < inc.minPrice {
			inc.minPrice = k
		}
		if k > inc.maxPrice {
			inc.maxPrice = k
		}
	}
}

// The Point of Control (POC) is a term used in the context of Volume Profile analysis. It refers to the price level at which the most
// volume has been traded in a security over a specified period of time. The POC is typically identified by looking for the highest
// peak in the Volume Profile, and is considered to be an important level of support or resistance. It can be used by traders to
// identify potential entry and exit points for trades, or to confirm other technical analysis signals.

// Get Resistence Level by finding PoC
func (inc *VolumeProfile) PointOfControlAboveEqual(price float64, limit ...float64) (resultPrice float64, vol float64) {
	filter := inc.maxPrice
	if len(limit) > 0 {
		filter = limit[0]
	}
	if inc.Delta == 0 {
		panic("Delta for volumeprofile shouldn't be zero")
	}
	start := math.Round(price / inc.Delta)
	vol = math.Inf(-1)
	if start > filter {
		return 0, 0
	}
	for ; start <= filter; start += inc.Delta {
		abs := math.Abs(inc.profile[start])
		if vol < abs {
			vol = abs
			resultPrice = start
		}

	}
	return resultPrice, vol
}

// Get Support Level by finding PoC
func (inc *VolumeProfile) PointOfControlBelowEqual(price float64, limit ...float64) (resultPrice float64, vol float64) {
	filter := inc.minPrice
	if len(limit) > 0 {
		filter = limit[0]
	}
	if inc.Delta == 0 {
		panic("Delta for volumeprofile shouldn't be zero")
	}
	start := math.Round(price / inc.Delta)
	vol = math.Inf(-1)

	if start < filter {
		return 0, 0
	}

	for ; start >= filter; start -= inc.Delta {
		abs := math.Abs(inc.profile[start])
		if vol < abs {
			vol = abs
			resultPrice = start
		}
	}
	return resultPrice, vol
}
