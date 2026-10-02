package binancecsv

import (
	"io"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

// ReadKLineFile reads one kline archive or plain CSV into a slice.
//
// It is the direct, cache-free path for code that already has files on disk and
// just wants candles; the Source is the right entry point for anything that
// needs downloading, merging or non-kline data.
func ReadKLineFile(path, symbol string, interval types.Interval) ([]types.KLine, error) {
	reader, closer, err := archive.OpenCSV(path)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	decoder := newKLineDecoder(Config{})
	meta := RecordMeta{
		Exchange: types.ExchangeBinance,
		Symbol:   symbol,
		Dataset:  DatasetKLines,
		Interval: interval,
		File:     filepath.Base(path),
	}

	var (
		klines []types.KLine
		buf    []marketdata.Event
	)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		meta.Columns = reader.Columns()
		meta.LineNo = reader.Line()

		buf = buf[:0]
		buf, err = decoder.Decode(buf, record, meta)
		if err != nil {
			return nil, err
		}

		for i := range buf {
			if buf[i].KLine != nil {
				klines = append(klines, *buf[i].KLine)
			}
		}
	}

	return klines, nil
}

// ReadKLineDir reads every .csv, .zip and .gz under dir into one slice, sorted
// by start time.
//
// This is the replacement for csvsource.ReadAllKLineCsv, with the same
// signature. Unlike that function it also accepts the compressed archives as
// downloaded, so a directory does not have to be unpacked first, and it sorts
// the result rather than relying on the order the directory walk happens to
// produce.
func ReadKLineDir(dir, symbol string, interval types.Interval) ([]types.KLine, error) {
	var klines []types.KLine

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		switch filepath.Ext(path) {
		case ".csv", ".zip", ".gz":
		default:
			return nil
		}

		got, err := ReadKLineFile(path, symbol, interval)
		if err != nil {
			return err
		}

		klines = append(klines, got...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(klines, func(i, j int) bool {
		return klines[i].StartTime.Before(klines[j].StartTime.Time())
	})

	return klines, nil
}
