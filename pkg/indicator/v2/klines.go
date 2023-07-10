package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

const MaxNumOfKLines = 4_000

//go:generate callbackgen -type KLineStream
type KLineStream struct {
	updateCallbacks []func(k types.KLine)

	kLines []types.KLine
}

func (s *KLineStream) Length() int {
	return len(s.kLines)
}

func (s *KLineStream) Last(i int) *types.KLine {
	l := len(s.kLines)
	if i < 0 || l-1-i < 0 {
		return nil
	}

	return &s.kLines[l-1-i]
}

// AddSubscriber adds the subscriber function and push historical data to the subscriber
func (s *KLineStream) AddSubscriber(f func(k types.KLine)) {
	s.OnUpdate(f)

	if len(s.kLines) == 0 {
		return
	}

	// push historical klines to the subscriber
	for _, k := range s.kLines {
		f(k)
	}
}

func (s *KLineStream) BackFill(kLines []types.KLine) {
	for _, k := range kLines {
		s.kLines = append(s.kLines, k)
		s.EmitUpdate(k)
	}
}

// KLines creates a KLine stream that pushes the klines to the subscribers
func KLines(source types.Stream, symbol string, interval types.Interval) *KLineStream {
	s := &KLineStream{}

	source.OnKLineClosed(types.KLineWith(symbol, interval, func(k types.KLine) {
		s.kLines = append(s.kLines, k)
		s.EmitUpdate(k)

		if len(s.kLines) > MaxNumOfKLines {
			s.kLines = s.kLines[len(s.kLines)-1-MaxNumOfKLines:]
		}
	}))

	return s
}

type KLineSubscription interface {
	AddSubscriber(f func(k types.KLine))
	Length() int
	Last(i int) *types.KLine
}
