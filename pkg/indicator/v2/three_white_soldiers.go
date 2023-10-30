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
			isUpTrend = two.High > three.High &&
				one.High > two.High
			isAllBullish = three.Open < three.Close &&
				two.Open < two.Close &&
				one.Open < one.Close
			doesOpenWithinPreviousBody = three.Close > two.Open &&
				two.Open < three.High &&
				two.High > one.Open &&
				one.Open < two.Close
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
