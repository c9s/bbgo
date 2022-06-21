package backtest

import (
	"encoding/csv"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestKLineDumper(t *testing.T) {
	tempDir := os.TempDir()
	_ = os.Mkdir(tempDir, 0755)
	dumper := NewKLineDumper(tempDir)

	t1 := time.Now()
	err := dumper.Record(types.KLine{
		Exchange:       types.ExchangeBinance,
		Symbol:         "BTCUSDT",
		StartTime:      types.Time(t1),
		EndTime:        types.Time(t1.Add(time.Minute)),
		Interval:       types.Interval1m,
		Open:           fixedpoint.NewFromFloat(1000.0),
		High:           fixedpoint.NewFromFloat(2000.0),
		Low:            fixedpoint.NewFromFloat(3000.0),
		Close:          fixedpoint.NewFromFloat(4000.0),
		Volume:         fixedpoint.NewFromFloat(5000.0),
		QuoteVolume:    fixedpoint.NewFromFloat(6000.0),
		NumberOfTrades: 10,
		Closed:         true,
	})
	assert.NoError(t, err)

	err = dumper.Close()
	assert.NoError(t, err)

	filenames := dumper.Filenames()
	assert.NotEmpty(t, filenames)
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if assert.NoError(t, err) {
			reader := csv.NewReader(f)
			records, err2 := reader.Read()
			if assert.NoError(t, err2) {
				assert.NotEmptyf(t, records, "%v", records)
			}

		}
	}
}
