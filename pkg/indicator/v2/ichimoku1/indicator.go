package ichimoku

import (
	"fmt"

	"github.com/algo-boyz/alphakit/market"
	decimal "github.com/algo-boyz/decimal128"

	"github.com/algo-boyz/alphakit/ta/dec"
)

type IIchimokuIndicator interface {
	// out result array sort: [new day -to- old day]
	// one call per asset
	MakeIchimokuInPast(bars []*market.Kline, numberOfRead int) error

	//one call per asset
	PreAnalyseIchimoku(data []*IchimokuStatus) (*IchimokuStatus, error)

	//Analyse Trigger Cross tenken & kijon sen
	AnalyseTriggerCross(previous *IchimokuStatus, bars_only_25_bars_latest []*market.Kline) (*IchimokuStatus, error)

	GetLast() *IchimokuStatus
	GetList() []*IchimokuStatus
}

type IchimokuIndicator struct {
	line_helper    lineHelper
	series         []*market.Kline
	Ichimokus      []*IchimokuStatus
	ConversionLine []float64
	BaseLine       []float64
	LeadingSpanA   []float64
	LeadingSpanB   []float64
	laggingSpan    []float64
}

func NewIndicator() IIchimokuIndicator {
	xx := IchimokuIndicator{}
	xx.Ichimokus = make([]*IchimokuStatus, 0)
	xx.line_helper = NewLineHelper()
	return &xx
}

func (xx *IchimokuIndicator) loadbars(from int, to int) []*market.Kline {
	if len(xx.series) == 0 {
		return nil
	}
	if from < 0 {
		from = 0
	}
	if to > len(xx.series) {
		to = len(xx.series)
	}

	return xx.series[from:to]

}

func (o *IchimokuIndicator) GetList() []*IchimokuStatus {
	return o.Ichimokus
}

func (o *IchimokuIndicator) MakeIchimokuInPast(series []*market.Kline, numberOfRead int) error {
	length := len(series)
	if length == 0 {
		return ErrDataNotFill
	}

	if length < numberOfRead || length < 52 {
		return ErrNotEnoughData
	}

	fmt.Printf("Calc ichi from Last %v days on %d total candles\r\n", numberOfRead, len(series))
	o.series = series

	// descending
	bars_len := len(o.series)
	for day := 0; day < numberOfRead; day++ {

		from := bars_len - 52 - day
		to := bars_len - day

		ic, err := BuildIchimokuStatus(o.loadbars(from, to))
		if err != nil {
			return err
		}
		o.Put(ic)

	}

	for day_index := 0; day_index < o.NumberOfIchimokus(); day_index++ {
		item := o.Ichimokus[day_index]
		sen_a, sen_b := o.Calc_Cloud_InPast(day_index)
		item.Set_SenCo_A_Past(sen_a)
		item.Set_SenCo_B_Past(sen_b)
	}

	return nil

}

// analyse with two days
func (o *IchimokuIndicator) FindStatusin26BarPast() {

	for i := 0; i < o.NumberOfIchimokus(); i++ {
		item := o.Ichimokus[i]
		sen_a, sen_b := o.Calc_Cloud_InPast(i)
		item.Set_SenCo_A_Past(sen_a)
		item.Set_SenCo_B_Past(sen_b)
	}

}

// Analyse Trigger Cross tenken & kijon sen
// bars : contain with new bar
func (o *IchimokuIndicator) AnalyseTriggerCross(previous *IchimokuStatus, _52_bars_latest []*market.Kline) (*IchimokuStatus, error) {

	if len(_52_bars_latest) == 0 {
		return nil, ErrDataNotFill
	}

	if len(_52_bars_latest) < 52 {
		return nil, ErrNotEnoughData
	}

	newIchi, e := BuildIchimokuStatus(_52_bars_latest)

	if e != nil {
		return nil, e
	}
	sen_a_in_26_past, sen_b_in_26_past := o.Calc_Cloud_InPast(0)

	if sen_a_in_26_past.isNil || sen_b_in_26_past.isNil {
		return nil, ErrChikoStatus26InPastNotMade
	}

	newIchi.Set_SenCo_A_Past(sen_a_in_26_past)
	newIchi.Set_SenCo_B_Past(sen_b_in_26_past)

	if newIchi.SencoA_Past.isNil || newIchi.SencoB_Past.isNil {
		return nil, ErrChikoStatus26InPastNotMade
	}

	if !newIchi.bar.Start.After(previous.bar.Start) {
		return nil, ErrDateNotGreaterThanPrevious
	}
	has_collision1, intersection := o.CrossCheck(previous, newIchi)
	if has_collision1 == EInterSectionStatus_Find {
		newIchi.SetStatus(newIchi.CloudStatus(intersection))
		newIchi.SetCloudSwitching(o.isSwitchCloud(previous, newIchi))

		return newIchi, nil
	}
	return nil, nil

}

// analyse with two days
func (o *IchimokuIndicator) PreAnalyseIchimoku(data []*IchimokuStatus) (*IchimokuStatus, error) {

	if len(data) != 2 {
		return nil, ErrNotEnoughData
	}

	current := data[0]
	previous := data[1]

	line1_point_a := NewPoint(dec.New(previous.bar.Start.Unix()), previous.TenkenSen.valLine)
	line1_point_b := NewPoint(dec.New(current.bar.Start.Unix()), current.TenkenSen.valLine)

	line2_point_a := NewPoint(dec.New(previous.bar.Start.Unix()), previous.KijonSen.valLine)
	line2_point_b := NewPoint(dec.New(current.bar.Start.Unix()), current.KijonSen.valLine)

	has_collision1, intersection := o.line_helper.GetCollisionDetection(line1_point_a, line1_point_b, line2_point_a, line2_point_b)

	if has_collision1 == EInterSectionStatus(IchimokuStatus_NAN) {
		return nil, nil
	}

	Line_Eq_A := o.getLineEquation(line1_point_a, line1_point_b) // tenken
	Line_Eq_B := o.getLineEquation(line2_point_a, line2_point_b) //kijon

	if line1_point_a == line2_point_a {
		return nil, nil //paraller

	}

	if Line_Eq_A.Slope.Sub(Line_Eq_B.Slope).Cmp(decimal.Zero) == 0 {
		return nil, nil

	}

	if has_collision1 == EInterSectionStatus_Find {

		current.SetStatus(current.CloudStatus(intersection))

		current.SetCloudSwitching(o.isSwitchCloud(previous, current))
		return current, nil
	}
	return nil, nil
}

func (o *IchimokuIndicator) NumberOfIchimokus() int {
	return len(o.Ichimokus)
}

func (o *IchimokuIndicator) GetLast() *IchimokuStatus {

	if o.Ichimokus != nil && len(o.Ichimokus) == 0 {
		return nil
	}

	latest := o.Ichimokus[0]

	return latest
}
func (o *IchimokuIndicator) Put(v *IchimokuStatus) {
	o.Ichimokus = append(o.Ichimokus, v)
}

// --------------------------------------------------------------------------------------------------------
func (o *IchimokuIndicator) CrossCheck(previous, newIchi *IchimokuStatus) (EInterSectionStatus, decimal.Decimal) {
	line1_point_a := NewPoint(dec.New(previous.bar.Start.Unix()), previous.TenkenSen.valLine)
	line1_point_b := NewPoint(dec.New(newIchi.bar.Start.Unix()), newIchi.TenkenSen.valLine)

	line2_point_a := NewPoint(dec.New(previous.bar.Start.Unix()), previous.KijonSen.valLine)
	line2_point_b := NewPoint(dec.New(newIchi.bar.Start.Unix()), newIchi.KijonSen.valLine)

	has_collision1, intersection := o.line_helper.GetCollisionDetection(line1_point_a, line1_point_b, line2_point_a, line2_point_b)

	if has_collision1 == EInterSectionStatus(IchimokuStatus_NAN) {
		return EInterSectionStatus_NAN, decimal.Zero
	}

	Line_Eq_A := o.getLineEquation(line1_point_a, line1_point_b) // tenken
	Line_Eq_B := o.getLineEquation(line2_point_a, line2_point_b) //kijon

	if line1_point_a == line2_point_a {
		return EInterSectionStatus_NAN, intersection //paraller

	}

	if Line_Eq_A.Slope.Sub(Line_Eq_B.Slope).Cmp(decimal.Zero) == 0 {
		return EInterSectionStatus_NAN, intersection

	}
	return has_collision1, intersection
}

// analyse with two days
func (o *IchimokuIndicator) isSwitchCloud(previous, current *IchimokuStatus) bool {

	if previous.SenKoA_Shifted26.valLine.GreaterThanOrEqual(previous.SenKoB_Shifted26.valLine) &&
		current.SenKoA_Shifted26.valLine.GreaterThan(current.SenKoB_Shifted26.valLine) ||
		previous.SenKoA_Shifted26.valLine.LessThan(previous.SenKoB_Shifted26.valLine) &&
			current.SenKoA_Shifted26.valLine.GreaterThanOrEqual(current.SenKoB_Shifted26.valLine) {
		if current.TenkenSen.valLine.GreaterThanOrEqual(current.KijonSen.valLine) {
			return true
		}
	}
	return false
}

// find cloud  Span A,B in Past (26 day )
func (o *IchimokuIndicator) Calc_Cloud_InPast(current int) (Point, Point) {

	if o.NumberOfIchimokus() < 26 {
		return NewNilPoint(), NewNilPoint()
	}

	rem := o.NumberOfIchimokus() - current
	max := 26 //from 26 bar in past  (find Shift index)

	if rem < max {
		return NewNilPoint(), NewNilPoint()
	}

	index := current + 25
	c := o.Ichimokus[index]
	buff_senco_a := NewPoint(dec.New(c.bar.Start.Unix()/1000), c.SenKoA_Shifted26.valLine)
	buff_senco_b := NewPoint(dec.New(c.bar.Start.Unix()/1000), c.SenKoB_Shifted26.valLine)

	return buff_senco_a, buff_senco_b
}

func (o *IchimokuIndicator) getLineEquation(p1 Point, p2 Point) *Equation {
	eq := Equation{}
	eq.Slope = (p2.Y.Sub(p1.Y)).Mul(p2.X.Sub(p1.X))
	//eq.Intercept = (-1 * eq.Slope * p1.X) + p1.Y
	eq.Intercept = p1.Y.Sub(eq.Slope).Mul(p1.X)
	return &eq
}
