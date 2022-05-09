package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/multierr"

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
	files           map[symbolInterval]*os.File
	writers         map[symbolInterval]*csv.Writer
	filenames       map[symbolInterval]string
}

func NewKLineDumper(outputDirectory string) *KLineDumper {
	return &KLineDumper{
		OutputDirectory: outputDirectory,
		files:           make(map[symbolInterval]*os.File),
		writers:         make(map[symbolInterval]*csv.Writer),
		filenames:       make(map[symbolInterval]string),
	}
}

func (d *KLineDumper) Filenames() map[symbolInterval]string {
	return d.filenames
}

func (d *KLineDumper) formatFileName(symbol string, interval types.Interval) string {
	return filepath.Join(d.OutputDirectory, fmt.Sprintf("%s-%s.csv",
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
		f2, err := os.Create(filename)
		if err != nil {
			return err
		}

		w = csv.NewWriter(f2)
		d.files[si] = f2
		d.writers[si] = w
		d.filenames[si] = filename

		if err := w.Write(csvHeader); err != nil {
			return err
		}
	}

	if err := w.Write(d.encode(k)); err != nil {
		return err
	}

	return nil
}

func (d *KLineDumper) Close() error {
	for _, w := range d.writers {
		w.Flush()
	}

	var err error = nil
	for _, f := range d.files {
		err2 := f.Close()
		if err2 != nil {
			err = multierr.Append(err, err2)
		}
	}

	return err
}
