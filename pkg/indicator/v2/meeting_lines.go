package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

type MeetingLinesStream struct {
	*types.Float64Series

	window int
}

func MeetingLines(source KLineSubscription) *MeetingLinesStream {
	s := &MeetingLinesStream{
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
			three = source.Last(2)
			two   = source.Last(1)
			one   = source.Last(0)
		)

		// BEAR
		if three.Open < three.Close {
			if two.Open < two.Close {
				if one.Open > one.Close {
					if fixedpoint.Abs((two.Close-one.Close)/one.Close) < threshold {
						output = Bear
					}
				}
			}
		}

		// BULL
		if three.Open > three.Close {
			if two.Open > two.Close {
				if one.Open < one.Close {
					if fixedpoint.Abs((two.Close-one.Close)/one.Close) < threshold {
						output = Bull
					}
				}
			}
		}
		s.PushAndEmit(output)

	})

	return s
}

func (s *MeetingLinesStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfPattern)
}
