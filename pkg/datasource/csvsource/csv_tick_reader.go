package csvsource

import (
	"encoding/csv"
	"io"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var _ TickReader = (*CSVTickReader)(nil)

// CSVTickReader is a CSVTickReader that reads from a CSV file.
type CSVTickReader struct {
	csv     *csv.Reader
	decoder CSVTickDecoder
	klines  []types.KLine
}

// MakeCSVTickReader is a factory method type that creates a new CSVTickReader.
type MakeCSVTickReader func(csv *csv.Reader) *CSVTickReader

// NewCSVKLineReader creates a new CSVKLineReader with the default Binance decoder.
func NewCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BinanceCSVTickDecoder,
		klines:  []types.KLine{},
	}
}

// NewCSVTickReaderWithDecoder creates a new CSVKLineReader with the given decoder.
func NewCSVTickReaderWithDecoder(csv *csv.Reader, decoder CSVTickDecoder) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: decoder,
		klines:  []types.KLine{},
	}
}

// ReadAll reads all the KLines from the underlying CSV data.
func (r *CSVTickReader) ReadAll(interval types.Interval) (k []types.KLine, err error) {
	var i int
	for {
		tick, err := r.Read(i)
		if err == io.EOF {
			break
		}
		if err != nil {
			return k, err
		}
		i++ // used as jump logic inside decoder to skip csv headers in case
		if tick == nil {
			continue
		}
		r.CsvTickToKLines(tick, interval)
	}

	return k, nil
}

// Read reads the next KLine from the underlying CSV data.
func (r *CSVTickReader) Read(i int) (*CsvTick, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return nil, err
	}

	return r.decoder(rec, i)
}

// Convert ticks to KLine with interval
func (c *CSVTickReader) CsvTickToKLines(tick *CsvTick, interval types.Interval) {
	var (
		currentCandle = types.KLine{}
		high          = fixedpoint.Zero
		low           = fixedpoint.Zero
	)
	isOpen, t := c.detCandleStart(tick.Timestamp.Time(), interval)

	if isOpen {
		c.klines = append(c.klines, types.KLine{
			StartTime: types.NewTimeFromUnix(t.Unix(), 0),
			EndTime:   types.NewTimeFromUnix(t.Add(interval.Duration()).Unix(), 0),
			Open:      tick.Price,
			High:      tick.Price,
			Low:       tick.Price,
			Close:     tick.Price,
			Volume:    tick.HomeNotional,
		})
		return
	}

	currentCandle = c.klines[len(c.klines)-1]

	if tick.Price.Float64() > currentCandle.High.Float64() {
		high = tick.Price
	} else {
		high = currentCandle.High
	}

	if tick.Price.Float64() < currentCandle.Low.Float64() {
		low = tick.Price
	} else {
		low = currentCandle.Low
	}

	c.klines[len(c.klines)-1] = types.KLine{
		StartTime: currentCandle.StartTime,
		EndTime:   currentCandle.EndTime,
		Open:      currentCandle.Open,
		High:      high,
		Low:       low,
		Close:     tick.Price,
		Volume:    currentCandle.Volume.Add(tick.HomeNotional),
	}
}

func (c *CSVTickReader) detCandleStart(ts time.Time, interval types.Interval) (isOpen bool, t time.Time) {
	if len(c.klines) == 0 {
		return true, interval.Convert(ts)
	}
	var (
		current = c.klines[len(c.klines)-1]
		end     = current.EndTime.Time()
	)
	if ts.After(end) {
		return true, end
	}

	return false, t
}
