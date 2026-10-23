package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator/v2/bars"
	"github.com/c9s/bbgo/pkg/types"
)

// TickBarStream aggregates trades into fixed-tick-count KLines ("tick bars").
type TickBarStream = bars.TickBarStream

// VolumeBarStream aggregates trades into fixed-volume KLines ("volume bars").
type VolumeBarStream = bars.VolumeBarStream

// TickBars creates a TickBarStream that aggregates market trades of the given symbol from
// source into fixed-tick-count KLines, emitting a new bar every numTicks trades.
func TickBars(source types.Stream, symbol string, numTicks int) *TickBarStream {
	return bars.TickBars(source, symbol, numTicks)
}

// VolumeBars creates a VolumeBarStream that aggregates market trades of the given symbol from
// source into fixed-volume KLines, emitting a new bar every time barVolume is accumulated.
func VolumeBars(source types.Stream, symbol string, barVolume fixedpoint.Value) *VolumeBarStream {
	return bars.VolumeBars(source, symbol, barVolume)
}
