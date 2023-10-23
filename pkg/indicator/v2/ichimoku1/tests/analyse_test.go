package tests

import (
	"testing"
	"time"

	"github.com/algo-boyz/alphakit/market"
	"github.com/stretchr/testify/assert"

	"github.com/algo-boyz/alphakit/ta/ichimoku"
)

var (
	interval = time.Minute * 30
	//52 bar
	ts = []*market.Kline{
		{L: d(8110), H: d(8180), C: d(8160), O: d(8110), Volume: d(664372.00), Start: p(1667201400000)},
		{L: d(8100), H: d(8260), C: d(8200), O: d(8150), Volume: d(1241301.00), Start: p(1667205000000)},
		{L: d(8110), H: d(8450), C: d(8440), O: d(8170), Volume: d(2909458.00), Start: p(1667280600000)},
		{L: d(8310), H: d(8450), C: d(8360), O: d(8440), Volume: d(778238.00), Start: p(1667284200000)},
		{L: d(8240), H: d(8370), C: d(8260), O: d(8360), Volume: d(658420.00), Start: p(1667287800000)},
		{L: d(8240), H: d(8450), C: d(8440), O: d(8260), Volume: d(1814124.00), Start: p(1667291400000)},
		{L: d(8270), H: d(8440), C: d(8300), O: d(8440), Volume: d(1267103.00), Start: p(1667367000000)},
		{L: d(8270), H: d(8510), C: d(8510), O: d(8300), Volume: d(1821017.00), Start: p(1667370600000)},
		{L: d(8430), H: d(8540), C: d(8440), O: d(8510), Volume: d(559250.00), Start: p(1667374200000)},
		{L: d(8420), H: d(8470), C: d(8440), O: d(8440), Volume: d(544851.00), Start: p(1667377800000)},
		{L: d(8480), H: d(8730), C: d(8730), O: d(8550), Volume: d(4284720.00), Start: p(1667626200000)},
		{L: d(8730), H: d(8730), C: d(8730), O: d(8730), Volume: d(1382828.00), Start: p(1667629800000)},
		{L: d(8730), H: d(8730), C: d(8730), O: d(8730), Volume: d(1678201.00), Start: p(1667633400000)},
		{L: d(8730), H: d(8730), C: d(8730), O: d(8730), Volume: d(549277.00), Start: p(1667637000000)},
		{L: d(8800), H: d(9070), C: d(9060), O: d(8800), Volume: d(5342062.00), Start: p(1667712600000)},
		{L: d(9040), H: d(9070), C: d(9070), O: d(9060), Volume: d(8126959.00), Start: p(1667716200000)},
		{L: d(9070), H: d(9070), C: d(9070), O: d(9070), Volume: d(527101.00), Start: p(1667719800000)},
		{L: d(9070), H: d(9070), C: d(9070), O: d(9070), Volume: d(702521.00), Start: p(1667723400000)},
		{L: d(9160), H: d(9440), C: d(9430), O: d(9290), Volume: d(4409696.00), Start: p(1667799000000)},
		{L: d(9410), H: d(9490), C: d(9490), O: d(9420), Volume: d(7522839.00), Start: p(1667802600000)},
		{L: d(9490), H: d(9490), C: d(9490), O: d(9490), Volume: d(777299.00), Start: p(1667806200000)},
		{L: d(9490), H: d(9490), C: d(9490), O: d(9490), Volume: d(405416.00), Start: p(1667809800000)},
		{L: d(9300), H: d(9890), C: d(9530), O: d(9890), Volume: d(7097789.00), Start: p(1667885400000)},
		{L: d(9460), H: d(9570), C: d(9470), O: d(9520), Volume: d(3033312.00), Start: p(1667889000000)},
		{L: d(9380), H: d(9490), C: d(9410), O: d(9470), Volume: d(2714433.00), Start: p(1667892600000)},
		{L: d(9390), H: d(9490), C: d(9450), O: d(9420), Volume: d(3876877.00), Start: p(1667896200000)},
		{L: d(9250), H: d(9540), C: d(9410), O: d(9350), Volume: d(3448605.00), Start: p(1667971800000)},
		{L: d(9400), H: d(9840), C: d(9800), O: d(9410), Volume: d(6547559.00), Start: p(1667975400000)},
		{L: d(9640), H: d(9830), C: d(9650), O: d(9800), Volume: d(2416825.00), Start: p(1667979000000)},
		{L: d(9650), H: d(9860), C: d(9680), O: d(9700), Volume: d(2463503.00), Start: p(1667982600000)},
		{L: d(9640), H: d(9870), C: d(9800), O: d(9750), Volume: d(2000789.00), Start: p(1668231000000)},
		{L: d(9520), H: d(9800), C: d(9520), O: d(9780), Volume: d(3214849.00), Start: p(1668234600000)},
		{L: d(9520), H: d(9680), C: d(9620), O: d(9550), Volume: d(3019512.00), Start: p(1668238200000)},
		{L: d(9610), H: d(9810), C: d(9740), O: d(9640), Volume: d(2473212.00), Start: p(1668241800000)},
		{L: d(9450), H: d(9710), C: d(9530), O: d(9710), Volume: d(1455003.00), Start: p(1668317400000)},
		{L: d(9510), H: d(9700), C: d(9700), O: d(9520), Volume: d(1341450.00), Start: p(1668321000000)},
		{L: d(9520), H: d(9720), C: d(9650), O: d(9700), Volume: d(2922575.00), Start: p(1668324600000)},
		{L: d(9470), H: d(9650), C: d(9470), O: d(9650), Volume: d(907574.00), Start: p(1668328200000)},
		{L: d(9250), H: d(9620), C: d(9250), O: d(9510), Volume: d(1573592.00), Start: p(1668403800000)},
		{L: d(9220), H: d(9420), C: d(9380), O: d(9270), Volume: d(1372258.00), Start: p(1668407400000)},
		{L: d(9340), H: d(9530), C: d(9490), O: d(9380), Volume: d(3147032.00), Start: p(1668411000000)},
		{L: d(9370), H: d(9550), C: d(9370), O: d(9490), Volume: d(2153637.00), Start: p(1668414600000)},
		{L: d(9380), H: d(9750), C: d(9670), O: d(9450), Volume: d(1861478.00), Start: p(1668490200000)},
		{L: d(9580), H: d(9700), C: d(9650), O: d(9670), Volume: d(2890813.00), Start: p(1668493800000)},
		{L: d(9610), H: d(9700), C: d(9670), O: d(9610), Volume: d(1288957.00), Start: p(1668497400000)},
		{L: d(9630), H: d(9800), C: d(9730), O: d(9650), Volume: d(2413843.00), Start: p(1668501000000)},
		{L: d(9580), H: d(9780), C: d(9630), O: d(9750), Volume: d(803830.00), Start: p(1668576600000)},
		{L: d(9630), H: d(9720), C: d(9670), O: d(9650), Volume: d(699785.00), Start: p(1668580200000)},
		{L: d(9640), H: d(9700), C: d(9640), O: d(9700), Volume: d(393592.00), Start: p(1668583800000)},
		{L: d(9580), H: d(9660), C: d(9630), O: d(9640), Volume: d(1443871.00), Start: p(1668587400000)},
		{L: d(9300), H: d(9600), C: d(9370), O: d(9510), Volume: d(3845936.00), Start: p(1668835800000)},
		{L: d(9310), H: d(9380), C: d(9330), O: d(9380), Volume: d(1380628.00), Start: p(1668839400000)},
	}
)

func Test_26DayinThePastNotExist(t *testing.T) {

	indicator := ichimoku.NewIndicator()

	indicator.MakeIchimokuInPast(ts, 1)

	_, e1 := indicator.AnalyseTriggerCross(indicator.GetLast(), ts)

	assert.Equal(t, e1, ichimoku.ErrChikoStatus26InPastNotMade)

}
func TestInside(t *testing.T) {

	indicator := ichimoku.NewIndicator()

	today := ichimoku.NewIchimokuStatus(
		ichimoku.NewValue(d(8705)),
		ichimoku.NewValue(d(8710)),
		ichimoku.NewValue(d(8707)),
		ichimoku.NewValue(d(8930)),
		&market.Kline{},
		&market.Kline{L: d(8400), H: d(8460), C: d(8440), O: d(8440), Volume: d(906352), Start: p(1664699400000)})

	yesterday := ichimoku.NewIchimokuStatus(
		ichimoku.NewValue(d(8720)),
		ichimoku.NewValue(d(8710)),
		ichimoku.NewValue(d(8715)),
		ichimoku.NewValue(d(8940)),
		&market.Kline{},
		&market.Kline{L: d(8430), H: d(8480), C: d(8450), O: d(8460), Volume: d(652416), Start: p(1664695800000)})

	lines_result := make([]*ichimoku.IchimokuStatus, 2)
	lines_result[0] = today //today
	lines_result[1] = yesterday

	a, e := indicator.PreAnalyseIchimoku(lines_result)
	assert.Empty(t, e)
	assert.Equal(t, a.Status, ichimoku.IchimokuStatus_Cross_Above)

}

func p(v int64) time.Time {
	return time.UnixMilli(v).UTC()
}
