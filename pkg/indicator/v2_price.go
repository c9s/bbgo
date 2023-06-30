package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

type KLineSubscription interface {
	AddSubscriber(f func(k types.KLine))
	Length() int
	Last(i int) *types.KLine
}

type PriceStream struct {
	*Float64Series

	mapper KLineValueMapper
}

func Price(source KLineSubscription, mapper KLineValueMapper) *PriceStream {
	s := &PriceStream{
		Float64Series: NewFloat64Series(),
		mapper:        mapper,
	}

	if source != nil {
		source.AddSubscriber(func(k types.KLine) {
			v := s.mapper(k)
			s.PushAndEmit(v)
		})
	}
	return s
}

func (s *PriceStream) PushAndEmit(v float64) {
	s.slice.Push(v)
	s.EmitUpdate(v)
}

func ClosePrices(source KLineSubscription) *PriceStream {
	return Price(source, KLineClosePriceMapper)
}

func LowPrices(source KLineSubscription) *PriceStream {
	return Price(source, KLineLowPriceMapper)
}

func HighPrices(source KLineSubscription) *PriceStream {
	return Price(source, KLineHighPriceMapper)
}

func OpenPrices(source KLineSubscription) *PriceStream {
	return Price(source, KLineOpenPriceMapper)
}

func Volumes(source KLineSubscription) *PriceStream {
	return Price(source, KLineVolumeMapper)
}
