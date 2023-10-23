package tests

import (
	"testing"

	decimal "github.com/algo-boyz/decimal128"
	"github.com/stretchr/testify/assert"

	"github.com/algo-boyz/alphakit/ta/dec"
	"github.com/algo-boyz/alphakit/ta/ichimoku"
)

func i(v int64) decimal.Decimal {
	return dec.New(v)
}

func d(v float64) decimal.Decimal {
	return dec.New(v)
}

func TestCheckCloud_Above(t *testing.T) {

	h1_above_ := []ichimoku.Point{
		{X: i(1667802600), Y: d(8762.00)},
		{X: i(1667806200), Y: d(8882.00)},
		{X: i(1667809800), Y: d(8908.00)},
		{X: i(1667885400), Y: d(8928.00)},
		{X: i(1667889000), Y: d(9152.00)},
		{X: i(1667892600), Y: d(9222.00)},
		{X: i(1667896200), Y: d(9238.00)},
		{X: i(1667971800), Y: d(9238.00)},
		{X: i(1667975400), Y: d(9260.00)},
		{X: i(1667979000), Y: d(9285.00)},
		{X: i(1667982600), Y: d(9318.00)},
		{X: i(1668231000), Y: d(9318.00)},
		{X: i(1668234600), Y: d(9318.00)},
		{X: i(1668238200), Y: d(9320.00)},
		{X: i(1668241800), Y: d(9320.00)},
		{X: i(1668317400), Y: d(9358.00)},
		{X: i(1668321000), Y: d(9358.00)},
		{X: i(1668324600), Y: d(9410.00)},
		{X: i(1668328200), Y: d(9485.00)},
		{X: i(1668403800), Y: d(9485.00)},
		{X: i(1668407400), Y: d(9435.00)},
		{X: i(1668411000), Y: d(9430.00)},
		{X: i(1668414600), Y: d(9490.00)},
		{X: i(1668490200), Y: d(9498.00)},
		{X: i(1668493800), Y: d(9482.00)},
	}
	line_helper := ichimoku.NewLineHelper()
	point_from_price := ichimoku.NewPoint(dec.New(1668497400/1000), d(9670))
	res_senko_a, err := line_helper.GetCollisionWithLine(point_from_price, h1_above_)

	//fmt.Println("senko A", res_senko_a, "err:", err)
	assert.Equal(t, res_senko_a, ichimoku.EPointLocation_above)
	assert.Empty(t, err)

}
func TestCheckCloud_below(t *testing.T) {

	h1_above_ := []ichimoku.Point{
		{X: i(1666589400), Y: d(8660.00)},
		{X: i(1666593000), Y: d(8660.00)},
		{X: i(1666596600), Y: d(8618.00)},
		{X: i(1666600200), Y: d(8555.00)},
		{X: i(1666675800), Y: d(8548.00)},
		{X: i(1666679400), Y: d(8502.00)},
		{X: i(1666683000), Y: d(8435.00)},
		{X: i(1666686600), Y: d(8435.00)},
		{X: i(1666762200), Y: d(8430.00)},
		{X: i(1666765800), Y: d(8360.00)},
		{X: i(1666769400), Y: d(8300.00)},
		{X: i(1666773000), Y: d(8282.00)},
		{X: i(1667021400), Y: d(8282.00)},
		{X: i(1667025000), Y: d(8282.00)},
		{X: i(1667028600), Y: d(8240.00)},
		{X: i(1667032200), Y: d(8160.00)},
		{X: i(1667107800), Y: d(8120.00)},
		{X: i(1667111400), Y: d(8120.00)},
		{X: i(1667115000), Y: d(8112.00)},
		{X: i(1667118600), Y: d(8110.00)},
		{X: i(1667194200), Y: d(8098.00)},
		{X: i(1667197800), Y: d(8100.00)},
		{X: i(1667201400), Y: d(8060.00)},
		{X: i(1667205000), Y: d(8052.00)},
		{X: i(1667280600), Y: d(8055.00)},
	}
	line_helper := ichimoku.NewLineHelper()
	point_from_price := ichimoku.NewPoint(dec.New(1667284200/1000), d(8360))
	res_senko_a, err := line_helper.GetCollisionWithLine(point_from_price, h1_above_)

	assert.Equal(t, res_senko_a, ichimoku.EPointLocation_below)
	assert.Empty(t, err)

}
func TestCheckCloud_below1(t *testing.T) {
	// h1 shegoya Tue Oct  2022 18 10:00:00

	h1_above_ := []ichimoku.Point{
		{X: i(1665819000), Y: d(8745.00)},
		{X: i(1665822600), Y: d(8750.00)},
		{X: i(1665898200), Y: d(8750.00)},
		{X: i(1665901800), Y: d(8730.00)},
		{X: i(1665905400), Y: d(8725.00)},
		{X: i(1665909000), Y: d(8712.00)},
		{X: i(1665984600), Y: d(8692.00)},
		{X: i(1665988200), Y: d(8692.00)},
		{X: i(1665991800), Y: d(8680.00)},
		{X: i(1665995400), Y: d(8680.00)},
		{X: i(1666071000), Y: d(8680.00)},
	}
	line_helper := ichimoku.NewLineHelper()
	point_from_price := ichimoku.NewPoint(dec.New(1666074600/1000), d(8650))
	res_senko_a, err := line_helper.GetCollisionWithLine(point_from_price, h1_above_)

	assert.Equal(t, res_senko_a, ichimoku.EPointLocation_below)
	assert.Empty(t, err)

}
func TestCheckCloud_below3(t *testing.T) {
	//|2022 Tue Nov 1 10:00:00
	h1_above_ := []ichimoku.Point{
		{X: i(1666589400), Y: d(8660.00)},
		{X: i(1666593000), Y: d(8660.00)},
		{X: i(1666596600), Y: d(8618.00)},
		{X: i(1666600200), Y: d(8555.00)},
		{X: i(1666675800), Y: d(8548.00)},
		{X: i(1666679400), Y: d(8502.00)},
		{X: i(1666683000), Y: d(8435.00)},
		{X: i(1666686600), Y: d(8435.00)},
		{X: i(1666762200), Y: d(8430.00)},
		{X: i(1666765800), Y: d(8360.00)},
		{X: i(1666769400), Y: d(8300.00)},
		{X: i(1666773000), Y: d(8282.00)},
		{X: i(1667021400), Y: d(8282.00)},
		{X: i(1667025000), Y: d(8282.00)},
		{X: i(1667028600), Y: d(8240.00)},
		{X: i(1667032200), Y: d(8160.00)},
		{X: i(1667107800), Y: d(8120.00)},
		{X: i(1667111400), Y: d(8120.00)},
		{X: i(1667115000), Y: d(8112.00)},
		{X: i(1667118600), Y: d(8110.00)},
		{X: i(1667194200), Y: d(8098.00)},
		{X: i(1667197800), Y: d(8100.00)},
		{X: i(1667201400), Y: d(8060.00)},
		{X: i(1667205000), Y: d(8052.00)},
		{X: i(1667280600), Y: d(8055.00)},
	}
	line_helper := ichimoku.NewLineHelper()
	point_from_price := ichimoku.NewPoint(dec.New(1667284200/1000), dec.New(8360))
	res_senko_a, err := line_helper.GetCollisionWithLine(point_from_price, h1_above_)

	//fmt.Println("senko A", res_senko_a, "err:", err)
	assert.Equal(t, res_senko_a, ichimoku.EPointLocation_below)
	assert.Empty(t, err)

}
