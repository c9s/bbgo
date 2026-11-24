package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ThreeWhiteSoldiersStream struct {
	*types.Float64Series

	window int
}

func ThreeWhiteSoldiers(source KLineSubscription) *ThreeWhiteSoldiersStream {
	s := &ThreeWhiteSoldiersStream{
		Float64Series: types.NewFloat64Series(),
		window:        3,
	}

	source.AddSubscriber(func(kLine types.KLine) {
		var (
			i      = source.Length()
			output = Neutral
		)
		if i < s.window {
			s.PushAndEmit(output)
			return
		}
		var (
			three     = source.Last(2)
			two       = source.Last(1)
			one       = source.Last(0)
			isUpTrend = two.High.Float64() > three.High.Float64() &&
				one.High.Float64() > two.High.Float64()
			isAllBullish = three.Open.Float64() < three.Close.Float64() &&
				two.Open.Float64() < two.Close.Float64() &&
				one.Open.Float64() < one.Close.Float64()
			doesOpenWithinPreviousBody = three.Close.Float64() > two.Open.Float64() &&
				two.Open.Float64() < three.High.Float64() &&
				two.High.Float64() > one.Open.Float64() &&
				one.Open.Float64() < two.Close.Float64()
		)

		if isUpTrend && isAllBullish && doesOpenWithinPreviousBody {
			output = Bull
		}

		s.PushAndEmit(output)

	})

	return s
}

func (s *ThreeWhiteSoldiersStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}
