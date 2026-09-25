package indicator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_calculateRSI(t *testing.T) {
	// test case from https://school.stockcharts.com/doku.php?id=technical_indicators:relative_strength_index_rsi
	buildKLines := func(prices []fixedpoint.Value) (kLines []types.KLine) {
		for _, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p})
		}
		return kLines
	}
	var data = []byte(`[44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64, 46.21, 46.25, 45.71, 46.45, 45.78, 45.35, 44.03, 44.18, 44.22, 44.57, 43.42, 42.66, 43.13]`)
	var values []fixedpoint.Value
	_ = json.Unmarshal(data, &values)

	tests := []struct {
		name   string
		kLines []types.KLine
		window int
		want   floats.Slice
	}{
		{
			name:   "RSI",
			kLines: buildKLines(values),
			window: 14,
			want: floats.Slice{
				70.46413502109704,
				66.24961855355505,
				66.48094183471265,
				69.34685316290864,
				66.29471265892624,
				57.91502067008556,
				62.88071830996241,
				63.208788718287764,
				56.01158478954758,
				62.33992931089789,
				54.67097137765515,
				50.386815195114224,
				40.01942379131357,
				41.49263540422282,
				41.902429678458105,
				45.499497238680405,
				37.32277831337995,
				33.090482572723396,
				37.78877198205783,
			},
		},
		// The cases below use the same stockcharts price series as the window=14 case.
		// Their expected values come from Wilder's smoothing (previous average weighted
		// by (n-1)/n), independently computed and cross-checked against the window=14
		// values above, so a single hard-coded rule has to satisfy every window.
		{
			name:   "RSI window=2",
			kLines: buildKLines(values),
			window: 2,
			want: floats.Slice{
				19.35483870967616,
				4.316546762589596,
				68.8524590163934,
				83.91777509068923,
				89.43606036536939,
				94.17433201927291,
				97.32448199557435,
				98.34638816362053,
				61.284574262028485,
				75.10834371108363,
				23.901843602872333,
				76.03333017139376,
				76.03333017139376,
				35.443998370861664,
				42.070727005787234,
				83.9107175070227,
				48.72128640927929,
				13.684412362407812,
				64.23574677625598,
				66.9521933338165,
				21.946038667240998,
				72.53905338776386,
				33.37075432141167,
				19.710029350851954,
				5.6101290028232995,
				18.81023399398518,
				24.445459374449115,
				65.88407336720599,
				14.309673890014565,
				7.032940786041607,
				42.92847638447948,
			},
		},
		{
			name:   "RSI window=7",
			kLines: buildKLines(values),
			window: 7,
			want: floats.Slice{
				70.30075187969925,
				74.92063492063494,
				77.27708533077657,
				71.10631884626227,
				72.96232313889763,
				59.56960157019129,
				69.86447921733948,
				69.86447921733948,
				61.02551404758972,
				61.632279722674404,
				68.80836888834145,
				62.03962911246165,
				45.94376438986956,
				58.33710127716063,
				59.10472348509058,
				45.81095879241771,
				60.14317246667896,
				47.00970965076617,
				40.403500072300986,
				26.87674317178258,
				29.98440393307277,
				30.89806227920205,
				39.021182425630194,
				26.899681971523222,
				21.701917400861532,
				31.28203430346599,
			},
		},
		{
			name:   "RSI window=21",
			kLines: buildKLines(values),
			window: 21,
			want: floats.Slice{
				64.02349486049927,
				59.102616239663824,
				63.17520765057338,
				57.711699906073605,
				54.53356916150554,
				46.312940615953636,
				47.26152355298069,
				47.521149465964264,
				49.79201091166993,
				43.324217422802235,
				39.742005244692294,
				42.81241207269459,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := RSI{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			rsi.CalculateAndUpdate(tt.kLines)
			assert.Equal(t, len(rsi.Values), len(tt.want))
			for i, v := range rsi.Values {
				assert.InDelta(t, v, tt.want[i], Delta)
			}
		})
	}
}
