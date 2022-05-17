package backtest

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/types"
)

const DateFormat = "2006-01-02T15:04"

type symbolInterval struct {
	Symbol   string
	Interval types.Interval
}

// KLineDumper dumps the received kline data into a folder for the backtest report to load the charts.
type KLineDumper struct {
	OutputDirectory string
	writers         map[symbolInterval]*tsv.Writer
	filenames       map[symbolInterval]string
}

func NewKLineDumper(outputDirectory string) *KLineDumper {
	return &KLineDumper{
		OutputDirectory: outputDirectory,
		writers:         make(map[symbolInterval]*tsv.Writer),
		filenames:       make(map[symbolInterval]string),
	}
}

func (d *KLineDumper) Filenames() map[symbolInterval]string {
	return d.filenames
}

func (d *KLineDumper) formatFileName(symbol string, interval types.Interval) string {
	return filepath.Join(d.OutputDirectory, fmt.Sprintf("%s-%s.tsv",
		symbol,
		interval))
}

var csvHeader = []string{"date", "startTime", "endTime", "interval", "open", "high", "low", "close", "volume"}

func (d *KLineDumper) encode(k types.KLine) []string {
	return []string{
		time.Time(k.StartTime).Format(time.ANSIC), // ANSIC date - for javascript to parse (this works with Date.parse(date_str)
		strconv.FormatInt(k.StartTime.Unix(), 10),
		strconv.FormatInt(k.EndTime.Unix(), 10),
		k.Interval.String(),
		k.Open.String(),
		k.High.String(),
		k.Low.String(),
		k.Close.String(),
		k.Volume.String(),
	}
}

func (d *KLineDumper) Record(k types.KLine) error {
	si := symbolInterval{Symbol: k.Symbol, Interval: k.Interval}

	w, ok := d.writers[si]
	if !ok {
		filename := d.formatFileName(k.Symbol, k.Interval)
		w2, err := tsv.NewWriterFile(filename)
		if err != nil {
			return err
		}
		w = w2

		d.writers[si] = w2
		d.filenames[si] = filename

		if err2 := w2.Write(csvHeader); err2 != nil {
			return err2
		}
	}

	return w.Write(d.encode(k))
}

func (d *KLineDumper) Close() error {
	var err error = nil
	for _, w := range d.writers {
		w.Flush()
		err2 := w.Close()
		if err2 != nil {
			err = multierr.Append(err, err2)
		}
	}

	return err
}
