package types

import (
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Direction int

const DirectionUp = 1
const DirectionNone = 0
const DirectionDown = -1

type CandleStickQueryOptions struct {
	Typename string
	Limit int
	StartTime *time.Time
	EndTime *time.Time
}

type CandleStick interface {
	GetType() string
	GetStartTime() Time
	GetEndTime() Time
	GetInterval() Interval
	Mid() fixedpoint.Value
	BounceUp() bool
	BounceDown() bool
	Direction() Direction
	GetHigh() fixedpoint.Value
	GetLow() fixedpoint.Value
	GetOpen() fixedpoint.Value
	GetClose() fixedpoint.Value
	GetMaxChange() fixedpoint.Value
	GetChange() fixedpoint.Value
	GetAmplification() fixedpoint.Value
	GetThickness() fixedpoint.Value
	GetUpperShadowRatio() fixedpoint.Value
	GetUpperShadowHeight() fixedpoint.Value
	GetLowerShadowRatio() fixedpoint.Value
	GetLowerShadowHeight() fixedpoint.Value
	GetBody() fixedpoint.Value
	Color() string
	String() string
	PlainText() string
	SlackAttachment() slack.Attachment
}

type CandleStickWindow interface {
	CandleStick
	ReduceClose() fixedpoint.Value
	First() CandleStick
	Last() CandleStick
	AllDrop() bool
	AllRise() bool
	GetTrend() int
	Take(size int) CandleStickWindow
	Tail(size int) CandleStickWindow
}

type CandleStickWindowPointer interface {
	Add(line CandleStick)
	Truncate(size int)
}

type CandleStickCallback func(c CandleStick)
