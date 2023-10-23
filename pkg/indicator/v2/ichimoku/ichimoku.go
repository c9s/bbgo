package ichimoku

import (
	"github.com/algo-boyz/decimal"
	"github.com/thecolngroup/gou/dec"

	"github.com/c9s/bbgo/pkg/types"
)

// type IIchimokuIndicator interface {
// 	// out result array sort: [new day -to- old day]
// 	// one call per asset
// 	MakeIchimokuInPast(bars []types.KLine, numberOfRead int) error

// 	//one call per asset
// 	PreAnalyseIchimoku(data []*IchimokuStatus) (*IchimokuStatus, error)

// 	//Analyse Trigger Cross tenken & kijon sen
// 	AnalyseTriggerCross(previous *IchimokuStatus, bars_only_25_bars_latest []types.KLine) (*IchimokuStatus, error)

// 	GetLast() *IchimokuStatus
// 	GetList() []*IchimokuStatus
// }

func (s *IchimokuStream) GetList() []*IchimokuStatus {
	return s.Ichimokus
}

// analyse with two days
func (s *IchimokuStream) FindStatusin26BarPast() {

	for i := 0; i < s.NumberOfIchimokus(); i++ {
		item := s.Ichimokus[i]
		sen_a, sen_b := s.calc_Cloud_InPast(i)
		item.Set_SenCo_A_Past(sen_a)
		item.Set_SenCo_B_Past(sen_b)
	}

}

// Analyse Trigger Cross tenken & kijon sen
// bars : contain with new bar
func (s *IchimokuStream) AnalyseTriggerCross(previous *IchimokuStatus, _52_bars_latest []types.KLine) (*IchimokuStatus, error) {

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
	sen_a_in_26_past, sen_b_in_26_past := s.calc_Cloud_InPast(0)

	if sen_a_in_26_past.isNil || sen_b_in_26_past.isNil {
		return nil, ErrChikoStatus26InPastNotMade
	}

	newIchi.Set_SenCo_A_Past(sen_a_in_26_past)
	newIchi.Set_SenCo_B_Past(sen_b_in_26_past)

	if newIchi.SencoA_Past.isNil || newIchi.SencoB_Past.isNil {
		return nil, ErrChikoStatus26InPastNotMade
	}

	if !newIchi.bar.StartTime.After(previous.bar.StartTime.Time()) {
		return nil, ErrDateNotGreaterThanPrevious
	}
	has_collision1, intersection := s.CrossCheck(previous, newIchi)
	if has_collision1 == EInterSectionStatus_Find {
		newIchi.SetStatus(newIchi.CloudStatus(intersection))
		newIchi.SetCloudSwitching(s.isSwitchCloud(previous, newIchi))

		return newIchi, nil
	}
	return nil, nil

}

// analyse with two days
func (s *IchimokuStream) PreAnalyseIchimoku(data []*IchimokuStatus) (*IchimokuStatus, error) {

	if len(data) != 2 {
		return nil, ErrNotEnoughData
	}

	current := data[0]
	previous := data[1]

	line1_point_a := NewPoint(dec.New(previous.bar.StartTime.Unix()), previous.TenkenSen.valLine)
	line1_point_b := NewPoint(dec.New(current.bar.StartTime.Unix()), current.TenkenSen.valLine)

	line2_point_a := NewPoint(dec.New(previous.bar.StartTime.Unix()), previous.KijonSen.valLine)
	line2_point_b := NewPoint(dec.New(current.bar.StartTime.Unix()), current.KijonSen.valLine)

	has_collision1, intersection := s.line_helper.GetCollisionDetection(line1_point_a, line1_point_b, line2_point_a, line2_point_b)

	if has_collision1 == EInterSectionStatus(IchimokuStatus_NAN) {
		return nil, nil
	}

	Line_Eq_A := s.getLineEquation(line1_point_a, line1_point_b) // tenken
	Line_Eq_B := s.getLineEquation(line2_point_a, line2_point_b) //kijon

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

func (s *IchimokuStream) NumberOfIchimokus() int {
	return len(s.Ichimokus)
}

func (s *IchimokuStream) GetLast() *IchimokuStatus {

	if s.Ichimokus != nil && len(s.Ichimokus) == 0 {
		return nil
	}

	latest := s.Ichimokus[0]

	return latest
}

func (s *IchimokuStream) Put(v *IchimokuStatus) {
	o.Ichimokus = append(o.Ichimokus, v)
}

// --------------------------------------------------------------------------------------------------------
func (s *IchimokuStream) CrossCheck(previous, newIchi *IchimokuStatus) (EInterSectionStatus, decimal.Decimal) {
	line1_point_a := NewPoint(dec.New(previous.bar.StartTime.Unix()), previous.TenkenSen.valLine)
	line1_point_b := NewPoint(dec.New(newIchi.bar.StartTime.Unix()), newIchi.TenkenSen.valLine)

	line2_point_a := NewPoint(dec.New(previous.bar.StartTime.Unix()), previous.KijonSen.valLine)
	line2_point_b := NewPoint(dec.New(newIchi.bar.StartTime.Unix()), newIchi.KijonSen.valLine)

	has_collision1, intersection := s.line_helper.GetCollisionDetection(line1_point_a, line1_point_b, line2_point_a, line2_point_b)

	if has_collision1 == EInterSectionStatus(IchimokuStatus_NAN) {
		return EInterSectionStatus_NAN, decimal.Zero
	}

	Line_Eq_A := s.getLineEquation(line1_point_a, line1_point_b) // tenken
	Line_Eq_B := s.getLineEquation(line2_point_a, line2_point_b) //kijon

	if line1_point_a == line2_point_a {
		return EInterSectionStatus_NAN, intersection //paraller

	}

	if Line_Eq_A.Slope.Sub(Line_Eq_B.Slope).Cmp(decimal.Zero) == 0 {
		return EInterSectionStatus_NAN, intersection

	}
	return has_collision1, intersection
}

// analyse with two days
func (s *IchimokuStream) isSwitchCloud(previous, current *IchimokuStatus) bool {

	if previous.SenKoA_Shifted26.valLine >= previous.SenKoB_Shifted26.valLine &&
		current.SenKoA_Shifted26.valLine > current.SenKoB_Shifted26.valLine ||
		previous.SenKoA_Shifted26.valLine < previous.SenKoB_Shifted26.valLine &&
			current.SenKoA_Shifted26.valLine >= current.SenKoB_Shifted26.valLine {
		if current.TenkenSen.valLine >= current.KijonSen.valLine {
			return true
		}
	}
	return false
}

// find cloud  Span A,B in Past (26 day )
func (s *IchimokuStream) calc_Cloud_InPast(current int) (Point, Point) {

	if s.NumberOfIchimokus() < 26 {
		return NewNilPoint(), NewNilPoint()
	}

	rem := s.NumberOfIchimokus() - current
	max := 26 //from 26 bar in past  (find Shift index)

	if rem < max {
		return NewNilPoint(), NewNilPoint()
	}

	index := current + 25
	c := s.Ichimokus[index]
	buff_senco_a := NewPoint(dec.New(c.bar.StartTime.Unix()/1000), c.SenKoA_Shifted26.valLine)
	buff_senco_b := NewPoint(dec.New(c.bar.StartTime.Unix()/1000), c.SenKoB_Shifted26.valLine)

	return buff_senco_a, buff_senco_b
}

func (s *IchimokuStream) getLineEquation(p1 Point, p2 Point) *Equation {
	eq := Equation{}
	eq.Slope = (p2.Y.Sub(p1.Y)).Mul(p2.X.Sub(p1.X))
	//eq.Intercept = (-1 * eq.Slope * p1.X) + p1.Y
	eq.Intercept = p1.Y.Sub(eq.Slope).Mul(p1.X)
	return &eq
}
