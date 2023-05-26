package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type KLineSource interface {
	OnUpdate(f func(k types.KLine))
}

type PriceStream struct {
	types.SeriesBase
	Float64Updater

	slice  floats.Slice
	mapper KLineValueMapper
}

func Price(source KLineSource, mapper KLineValueMapper) *PriceStream {
	s := &PriceStream{
		mapper: mapper,
	}
	s.SeriesBase.Series = s.slice
	source.OnUpdate(func(k types.KLine) {
		v := s.mapper(k)
		s.slice.Push(v)
		s.EmitUpdate(v)
	})
	return s
}

func ClosePrices(source KLineSource) *PriceStream {
	return Price(source, KLineClosePriceMapper)
}

func LowPrices(source KLineSource) *PriceStream {
	return Price(source, KLineLowPriceMapper)
}

func HighPrices(source KLineSource) *PriceStream {
	return Price(source, KLineHighPriceMapper)
}

func OpenPrices(source KLineSource) *PriceStream {
	return Price(source, KLineOpenPriceMapper)
}
