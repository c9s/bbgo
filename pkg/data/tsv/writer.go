package tsv

import (
	"encoding/csv"
	"io"
	"os"
)

type Writer struct {
	file io.WriteCloser

	*csv.Writer
}

func NewWriterFile(filename string) (*Writer, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	return NewWriter(f), nil
}

func AppendWriterFile(filename string) (*Writer, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return NewWriter(f), nil
}

func NewWriter(file io.WriteCloser) *Writer {
	tsv := csv.NewWriter(file)
	tsv.Comma = '\t'
	return &Writer{
		Writer: tsv,
		file:   file,
	}
}

func (w *Writer) Close() error {
	w.Writer.Flush()
	return w.file.Close()
}
