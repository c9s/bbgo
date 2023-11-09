package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type PriceStream struct {
	*types.Float64Series

	mapper types.KLineValueMapper
}

func Price(source KLineSubscription, mapper types.KLineValueMapper) *PriceStream {
	s := &PriceStream{
		Float64Series: types.NewFloat64Series(),
		mapper:        mapper,
	}

	if source == nil {
		return s
	}

	source.AddSubscriber(func(k types.KLine) {
		v := s.mapper(k)
		s.PushAndEmit(v)
	})
	return s
}

// AddSubscriber adds the subscriber function and push historical data to the subscriber
func (s *PriceStream) AddSubscriber(f func(v float64)) {
	s.OnUpdate(f)

	if len(s.Slice) == 0 {
		return
	}

	// push historical value to the subscriber
	for _, v := range s.Slice {
		f(v)
	}
}

func (s *PriceStream) PushAndEmit(v float64) {
	s.Slice.Push(v)
	s.EmitUpdate(v)
}

func ClosePrices(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineClosePriceMapper)
}

func LowPrices(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineLowPriceMapper)
}

func HighPrices(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineHighPriceMapper)
}

func OpenPrices(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineOpenPriceMapper)
}

func Volumes(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineVolumeMapper)
}

func HLC3(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineHLC3Mapper)
}

func HLC3MulVolume(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineHLC3xVMapper)
}

func HL2(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineHL2Mapper)
}

func CloseMulVolume(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineCxVMapper)
}

func CloseSubOpen(source KLineSubscription) *PriceStream {
	return Price(source, types.KLineCOMapper)
}

func DiffClose(source KLineSubscription) *PriceStream {
	s := &PriceStream{
		Float64Series: types.NewFloat64Series(),
	}

	if source == nil {
		return s
	}

	source.AddSubscriber(func(k types.KLine) {
		if source.Length() > 0 {
			s.PushAndEmit(math.Log(k.Close.Div(source.Last(0).Close).Float64()))
		}
	})
	return s
}
