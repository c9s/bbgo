package csvsource

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func assertKLineEq(t *testing.T, exp, act types.KLine, name string) {
	assert.Equal(t, exp.StartTime, act.StartTime, name)
	assert.Equal(t, 0, exp.Open.Compare(act.Open), name)
	assert.Equal(t, 0, exp.High.Compare(act.High), name)
	assert.Equal(t, 0, exp.Low.Compare(act.Low), name)
	assert.Equal(t, 0, exp.Close.Compare(act.Close), name)
	assert.Equal(t, 0, exp.Volume.Compare(act.Volume), name)
}

func TestCSVKLineReader_ReadWithBinanceDecoder(t *testing.T) {
	tests := []struct {
		name string
		give string
		want types.KLine
		err  error
	}{
		{
			name: "Read DOHLCV",
			give: "1609459200000,28923.63000000,29031.34000000,28690.17000000,28995.13000000,2311.81144500",
			want: types.KLine{
				StartTime: types.NewTimeFromUnix(1609459200, 0),
				Open:      Number(28923.63),
				High:      Number(29031.34),
				Low:       Number(28690.17),
				Close:     Number(28995.13),
				// todo this should never happen >>
				// mustNewFromString and NewFromFloat have different values after parse
				Volume: fixedpoint.MustNewFromString("2311.81144500")},
			err: nil,
		},
		{
			name: "Read DOHLC",
			give: "1609459200000,28923.63000000,29031.34000000,28690.17000000,28995.13000000",
			want: types.KLine{
				StartTime: types.NewTimeFromUnix(1609459200, 0),
				Open:      Number(28923.63),
				High:      Number(29031.34),
				Low:       Number(28690.17),
				Close:     Number(28995.13),
				Volume:    Number(0)},
			err: nil,
		},
		{
			name: "Not enough columns",
			give: "1609459200000,28923.63000000,29031.34000000",
			want: types.KLine{},
			err:  ErrNotEnoughColumns,
		},
		{
			name: "Invalid time format",
			give: "23/12/2021,28923.63000000,29031.34000000,28690.17000000,28995.13000000",
			want: types.KLine{},
			err:  ErrInvalidTimeFormat,
		},
		{
			name: "Invalid price format",
			give: "1609459200000,sixty,29031.34000000,28690.17000000,28995.13000000",
			want: types.KLine{},
			err:  ErrInvalidPriceFormat,
		},
		{
			name: "Invalid volume format",
			give: "1609459200000,28923.63000000,29031.34000000,28690.17000000,28995.13000000,vol",
			want: types.KLine{},
			err:  ErrInvalidVolumeFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewBinanceCSVKLineReader(csv.NewReader(strings.NewReader(tt.give)))
			kline, err := reader.Read(time.Hour)
			assert.Equal(t, tt.err, err)
			if err == nil {
				spew.Dump(tt.want)
				spew.Dump(kline)
				assertKLineEq(t, tt.want, kline, tt.name)
			}
		})
	}
}

func TestCSVKLineReader_ReadAllWithDefaultDecoder(t *testing.T) {
	records := []string{
		"1609459200000,28923.63000000,29031.34000000,28690.17000000,28995.13000000,2311.81144500",
		"1609459300000,28928.63000000,30031.34000000,22690.17000000,28495.13000000,3000.00",
	}
	reader := NewCSVKLineReader(csv.NewReader(strings.NewReader(strings.Join(records, "\n"))))
	klines, err := reader.ReadAll(time.Hour)
	assert.NoError(t, err)
	assert.Len(t, klines, 2)
}

func TestCSVKLineReader_ReadWithMetaTraderDecoder(t *testing.T) {

	tests := []struct {
		name string
		give string
		want types.KLine
		err  error
	}{
		{
			name: "Read DOHLCV",
			give: "11/12/2008;16:00;779.527679;780.964756;777.527679;779.964756;5",
			want: types.KLine{
				StartTime: types.NewTimeFromUnix(time.Date(2008, 12, 11, 16, 0, 0, 0, time.UTC).Unix(), 0),
				Open:      Number(779.527679),
				High:      Number(780.964756),
				Low:       Number(777.527679),
				Close:     Number(779.964756),
				Volume:    Number(5)},
			err: nil,
		},
		{
			name: "Not enough columns",
			give: "1609459200000;28923.63000000;29031.34000000",
			want: types.KLine{},
			err:  ErrNotEnoughColumns,
		},
		{
			name: "Invalid time format",
			give: "23/12/2021;t;28923.63000000;29031.34000000;28690.17000000;28995.13000000",
			want: types.KLine{},
			err:  ErrInvalidTimeFormat,
		},
		{
			name: "Invalid price format",
			give: "11/12/2008;00:00;sixty;29031.34000000;28690.17000000;28995.13000000",
			want: types.KLine{},
			err:  ErrInvalidPriceFormat,
		},
		{
			name: "Invalid volume format",
			give: "11/12/2008;00:00;779.527679;780.964756;777.527679;779.964756;vol",
			want: types.KLine{},
			err:  ErrInvalidVolumeFormat,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMetaTraderCSVKLineReader(csv.NewReader(strings.NewReader(tt.give)))
			kline, err := reader.Read(time.Hour)
			assert.Equal(t, tt.err, err)
			assertKLineEq(t, tt.want, kline, tt.name)
		})
	}
}
