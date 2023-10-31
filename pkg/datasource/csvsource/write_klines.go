package csvsource

import (
	"encoding/csv"
	"os"
	"strconv"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

// WriteKLines writes csv to path.
func WriteKLines(path string, prices []types.KLine) (err error) {
	file, err := os.Create(path)
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
	for _, record := range prices {
		row := []string{strconv.Itoa(int(record.StartTime.Unix())), record.Open.String(), record.High.String(), record.Low.String(), record.Close.String(), record.Volume.String()}
		if err := w.Write(row); err != nil {
			return errors.Wrap(err, "writing record to file")
		}
	}
	if err != nil {
		return err
	}

	return nil
}
