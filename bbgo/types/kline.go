package types

import "github.com/slack-go/slack"

type KLine interface {
	GetTrend() int
	GetChange() float64
	GetMaxChange() float64
	GetThickness() float64

	Mid() float64
	GetOpen() float64
	GetClose() float64
	GetHigh() float64
	GetLow() float64

	BounceUp() bool
	BounceDown() bool
	GetUpperShadowRatio() float64
	GetLowerShadowRatio() float64

	SlackAttachment() slack.Attachment
}

