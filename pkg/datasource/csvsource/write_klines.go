package csvsource

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

// WriteKLines writes csv to path.
func WriteKLines(path, symbol string, klines []types.KLine) (err error) {
	if len(klines) == 0 {
		return fmt.Errorf("no klines to write")
	}
	from := klines[0].StartTime.Time()
	end := klines[len(klines)-1].EndTime.Time()
	to := ""
	if from.AddDate(0, 0, 1).After(end) {
		to = "-" + end.Format("2006-01-02")
	}

	path = fmt.Sprintf("%s/klines/%s",
		path,
		klines[0].Interval.String(),
	)

	fileName := fmt.Sprintf("%s/%s-%s%s.csv",
		path,
		symbol,
		from.Format("2006-01-02"),
		to,
	)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
	}

	file, err := os.Create(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer func() {
		err = file.Close()
		if err != nil {
			panic("failed to close file")
		}
	}()

	w := csv.NewWriter(file)
	defer w.Flush()

	// Using Write
	for _, kline := range klines {
		row := []string{
			fmt.Sprintf("%d", kline.StartTime.Unix()),
			kline.Open.String(),
			kline.High.String(),
			kline.Low.String(),
			kline.Close.String(),
			kline.Volume.String(),
		}
		if err := w.Write(row); err != nil {
			return errors.Wrap(err, "writing record to file")
		}
	}
	if err != nil {
		return err
	}

	return nil
}
