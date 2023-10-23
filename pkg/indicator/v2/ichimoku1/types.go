package ichimoku

import (
	"errors"

	decimal "github.com/algo-boyz/decimal128"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var (
	ErrBuildFailed                = errors.New("build failed")
	ErrNotEnoughData              = errors.New("not enough data")
	ErrDataNotFill                = errors.New("data not fill")
	ErrChikoStatus26InPastNotMade = errors.New("chiko status 26 in past not reached")
	ErrDateNotGreaterThanPrevious = errors.New("date is not greater than previous")
)

type Point struct {
	X, Y  fixedpoint.Value
	isNil bool
}

func NewPoint(x, y fixedpoint.Value) Point {
	p := Point{}
	p.X = x
	p.Y = y
	p.isNil = false
	return p
}
func NewNilPoint() Point {
	p := Point{}
	p.X = fixedpoint.NewFromInt(-1)
	p.Y = fixedpoint.NewFromInt(-1)
	p.isNil = true
	return p
}

type Equation struct {
	Slope     decimal.Decimal
	Intercept decimal.Decimal
}

type EInterSectionStatus int

const (
	EInterSectionStatus_NAN            EInterSectionStatus = 0
	EInterSectionStatus_Find           EInterSectionStatus = 1
	EInterSectionStatus_Parallel       EInterSectionStatus = 2
	EInterSectionStatus_Collision_Find EInterSectionStatus = 1
)

type ELine int

const (
	Line_Tenkan_sen  ELine = 9
	Line_kijon_sen   ELine = 26
	Line_spanPeriod  ELine = 52
	Line_chikoPeriod ELine = 26 //-26
)

type EIchimokuStatus int

const (
	IchimokuStatus_NAN          EIchimokuStatus = 0
	IchimokuStatus_Cross_Inside EIchimokuStatus = 1
	IchimokuStatus_Cross_Below  EIchimokuStatus = 2
	IchimokuStatus_Cross_Above  EIchimokuStatus = 3
	IchimokuStatus_overLab      EIchimokuStatus = 4
)
