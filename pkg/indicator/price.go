package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type KLineSubscription interface {
	AddSubscriber(f func(k types.KLine))
}

type PriceStream struct {
	types.SeriesBase
	Float64Updater

	slice  floats.Slice
	mapper KLineValueMapper
}

func Price(source KLineSubscription, mapper KLineValueMapper) *PriceStream {
	s := &PriceStream{
		mapper: mapper,
	}

	s.SeriesBase.Series = s.slice

	source.AddSubscriber(func(k types.KLine) {
		v := s.mapper(k)
		s.slice.Push(v)
		s.EmitUpdate(v)
	})
	return s
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
