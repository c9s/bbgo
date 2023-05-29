package indicator

import "github.com/c9s/bbgo/pkg/types"

const MaxNumOfKLines = 4_000

//go:generate callbackgen -type KLineStream
type KLineStream struct {
	updateCallbacks []func(k types.KLine)

	kLines []types.KLine
}

// AddSubscriber adds the subscriber function and push historical data to the subscriber
func (s *KLineStream) AddSubscriber(f func(k types.KLine)) {
	if len(s.kLines) > 0 {
		// push historical klines to the subscriber
		for _, k := range s.kLines {
			f(k)
		}
	}
	s.OnUpdate(f)
}

// KLines creates a KLine stream that pushes the klines to the subscribers
func KLines(source types.Stream) *KLineStream {
	s := &KLineStream{}

	source.OnKLineClosed(func(k types.KLine) {
		s.kLines = append(s.kLines, k)

		if len(s.kLines) > MaxNumOfKLines {
			s.kLines = s.kLines[len(s.kLines)-1-MaxNumOfKLines:]
		}
		s.EmitUpdate(k)
	})

	return s
}
