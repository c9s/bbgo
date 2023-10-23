package ichimoku

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type lineHelper struct {
}
type EPointLocation int

const (
	EPointLocation_NAN     EPointLocation = 0
	EPointLocation_above   EPointLocation = 1
	EPointLocation_below   EPointLocation = 2
	EPointLocation_overlap EPointLocation = 3
)

func NewLineHelper() lineHelper {
	f := lineHelper{}
	return f
}

// check Point above or below line
func (n *lineHelper) isAboveLine(point Point, line []Point) EPointLocation {
	zero := fixedpoint.Zero
	if len(line) < 2 {
		return EPointLocation_NAN
	}

	checkAboveOrBelow := func(p Point, line_p_a Point, line_p_b Point) EPointLocation {
		//var d = (_p_x- _x1) *(_y2-_y1)-(_p_y-_y1)*(_x2-_x1)
		// v := (p.X-line_p_a.X)*(line_p_b.Y-line_p_a.Y) - (p.Y-line_p_a.Y)*(line_p_b.X-line_p_a.X)
		yOverX := p.Y.Sub(line_p_a.Y).Mul(line_p_b.X.Sub(line_p_a.X))
		xOverY := p.X.Sub(line_p_a.X).Mul(line_p_b.Y.Sub(line_p_a.Y))
		v := xOverY.Sub(yOverX)

		if v.Compare(zero) == 0 {
			return EPointLocation_overlap
		} else if v > zero {
			return EPointLocation_above
		} else {
			return EPointLocation_below
		}
	}

	return checkAboveOrBelow(point, line[0], line[1])

}

func (n *lineHelper) positionPointInline(point Point, line []Point) fixedpoint.Value {
	zero := fixedpoint.Zero

	if len(line) < 2 {
		return zero
	}

	checkAboveOrBelow := func(p Point, line_p_a Point, line_p_b Point) fixedpoint.Value {
		//var d = (_p_x- _x1) *(_y2-_y1)-(_p_y-_y1)*(_x2-_x1)
		return (p.X.Sub(line_p_a.X)).Mul(line_p_b.Y.Sub(line_p_a.Y)).Sub(p.Y.Sub(line_p_a.Y)).Mul(line_p_b.X.Sub(line_p_a.X))
	}

	res := checkAboveOrBelow(point, line[0], line[1])

	return res

}

func (o *lineHelper) GetCollisionDetection(a Point, b Point, c Point, d Point) (EInterSectionStatus, fixedpoint.Value) {

	denominator := ((b.X.Sub(a.X)).Mul(d.Y.Sub(c.Y))).Sub((b.Y.Sub(a.Y)).Mul(d.X.Sub(c.X)))
	numerator1 := ((a.Y.Sub(c.Y)).Mul(d.X.Sub(c.X))).Sub((a.X.Sub(c.X)).Mul(d.Y.Sub(c.Y)))
	numerator2 := ((a.Y.Sub(c.Y)).Mul(b.X.Sub(a.X))).Sub((a.X.Sub(c.X)).Mul(b.Y.Sub(a.Y)))

	// Detect coincident lines (has a problem, read below)
	if denominator.Compare(fixedpoint.Zero) == 0 {
		return EInterSectionStatus_NAN, fixedpoint.Zero
	}
	r := numerator1.Div(denominator)
	s := numerator2.Div(denominator)
	zero := fixedpoint.Zero
	one := fixedpoint.One
	if r >= zero && r <= one &&
		s >= zero && s <= one {
		//	fmt.Printf("collision detec : a:%v , b:%v, c:%v ,d:%v ,r %v s %v\r\n", a, b, c, d, r, s)
		intersection := o.get_intersection_point(a, b, c, d)

		return EInterSectionStatus_Collision_Find, intersection
	}
	return EInterSectionStatus_NAN, fixedpoint.Zero
}

//line senco A or B
func (o *lineHelper) GetCollisionWithLine(price_point Point, line_clouds []Point) (EPointLocation, error) {
	zero := fixedpoint.Zero

	len_line_clouds := len(line_clouds)
	if len_line_clouds < 1 {
		return EPointLocation_NAN, ErrNotEnoughData
	}
	// Create a point at infinity, y is same as point p
	//x := time.Now().AddDate(0, -1, 0).Unix()
	//line1_a := NewPoint(decimal.NewFromInt(x), price_point.Y)
	//	line1_b := price_point
	//  line_b := NewPoint(0,price_point)
	//var ps EPointLocation = EPointLocation_NaN
	below := 0
	above := 0
	fmt.Println("___")
	fmt.Printf("Cloud : check point :x:%.0f y:%.0f \r\n", price_point.X.Float64(), price_point.Y.Float64())
	sum := fixedpoint.Zero
	for i := 1; i < len_line_clouds; i++ {
		line2_a := line_clouds[i-1]
		line2_b := line_clouds[i]
		//fmt.Printf("Cloud :x:%.0f y:%.0f ,\r\n", line2_a.X, line2_a.Y)
		fmt.Printf("%.0f,%.0f,\r\n", line2_a.X.Float64(), line2_a.Y.Float64())
		//res := o.GetCollisionDetection(line1_a, line1_b, line2_a, line2_b)
		buff := []Point{line2_a, line2_b}
		// res := o.isAboveLine(price_point, buff)
		c := o.positionPointInline(price_point, buff)
		if c > zero { //below
			below++
		} else {
			above++
		}

		sum = sum.Add(c)

	}
	v := sum.Div(fixedpoint.NewFromInt(int64(len_line_clouds)))

	if v > zero {
		return EPointLocation_below, nil // above
	} else {
		return EPointLocation_above, nil //below
	}

}
func (o *lineHelper) getLineEquation(p1 Point, p2 Point) *Equation {
	eq := Equation{}
	x := p1.X.Sub(p2.X)
	y := p1.Y.Sub(p2.Y)
	eq.Slope = y.Div(x)
	eq.Intercept = fixedpoint.One.Neg().Mul(eq.Slope).Mul(p1.X).Add(p1.Y)
	return &eq
}

func (o *lineHelper) get_intersection_point(line1_pointA Point, line1_pointB Point, line2_pointA Point, line2_pointB Point) fixedpoint.Value {

	tenken := o.getLineEquation(line1_pointA, line1_pointB)
	kijon := o.getLineEquation(line2_pointA, line2_pointB)
	intercept := kijon.Intercept.Sub(tenken.Intercept)
	slope := tenken.Slope.Sub(kijon.Slope)
	x_intersection := intercept.Div(slope)
	x := kijon.Slope.Mul(x_intersection)
	y_intersection := x.Add(kijon.Intercept)
	return y_intersection
}
