package replay

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// FileExtension is the recording file extension, gzipped JSON Lines.
const FileExtension = ".jsonl.gz"

// WriterConfig configures a Writer.
type WriterConfig struct {
	// Dir is where recordings are written.
	Dir string

	// Exchange and Symbols go into the header of each file.
	Exchange types.ExchangeName
	Symbols  []string
	Channels []types.Channel

	// Rotate is how often a new file is started. Hourly by default: a crash
	// then costs at most one hour, and a replay can seek by file name.
	Rotate time.Duration

	// Uncompressed writes plain .jsonl, which is easier to eyeball in a test.
	Uncompressed bool

	// Note is copied into the header, for recording what the capture was for.
	Note string
}

// Writer appends records to rotating recording files.
//
// It is safe for concurrent use, because a stream's callbacks can fire from
// several goroutines and dropping an event to a race would defeat the purpose.
type Writer struct {
	cfg WriterConfig

	mu       sync.Mutex
	file     *os.File
	gz       *gzip.Writer
	buf      *bufio.Writer
	enc      *json.Encoder
	period   time.Time
	filename string
	count    int64
}

// NewWriter creates the output directory and returns a Writer. No file is
// opened until the first record.
func NewWriter(cfg WriterConfig) (*Writer, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("replay: writer needs a directory")
	}
	if cfg.Rotate <= 0 {
		cfg.Rotate = time.Hour
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, err
	}

	return &Writer{cfg: cfg}, nil
}

// Count returns how many records have been written.
func (w *Writer) Count() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Filename returns the file currently being written, or "" before the first
// record.
func (w *Writer) Filename() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.filename
}

// WriteEvent encodes and appends an event. Event types the format does not
// carry are skipped.
func (w *Writer) WriteEvent(ev *marketdata.Event) error {
	record, ok := EncodeEvent(ev)
	if !ok {
		return nil
	}
	return w.Write(record)
}

// Write appends one record, rotating the file if the record falls into a new
// period.
func (w *Writer) Write(r Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	period := nanoTime(r.T).Truncate(w.cfg.Rotate)
	if w.enc == nil || !period.Equal(w.period) {
		if err := w.rotate(period); err != nil {
			return err
		}
	}

	if err := w.enc.Encode(r); err != nil {
		return err
	}
	w.count++
	return nil
}

// rotate closes the current file and opens the one for period.
func (w *Writer) rotate(period time.Time) error {
	if err := w.closeCurrent(); err != nil {
		return err
	}

	symbol := "multi"
	if len(w.cfg.Symbols) == 1 {
		symbol = w.cfg.Symbols[0]
	}

	ext := FileExtension
	if w.cfg.Uncompressed {
		ext = ".jsonl"
	}

	name := fmt.Sprintf("%s-%s-%s%s",
		w.cfg.Exchange, symbol, period.UTC().Format("20060102T15"), ext)
	path := filepath.Join(w.cfg.Dir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	var out io.Writer = f
	if !w.cfg.Uncompressed {
		w.gz = gzip.NewWriter(f)
		out = w.gz
	}
	w.buf = bufio.NewWriterSize(out, 64*1024)
	w.enc = json.NewEncoder(w.buf)
	w.file = f
	w.period = period
	w.filename = path

	return w.enc.Encode(Header{
		Version:   FormatVersion,
		Exchange:  w.cfg.Exchange,
		Symbols:   w.cfg.Symbols,
		Channels:  w.cfg.Channels,
		StartedAt: period.UnixNano(),
		Note:      w.cfg.Note,
	})
}

func (w *Writer) closeCurrent() error {
	if w.file == nil {
		return nil
	}

	var err error
	if w.buf != nil {
		err = multierr.Append(err, w.buf.Flush())
	}
	if w.gz != nil {
		err = multierr.Append(err, w.gz.Close())
	}
	err = multierr.Append(err, w.file.Close())

	w.file, w.gz, w.buf, w.enc = nil, nil, nil, nil
	return err
}

// Flush pushes buffered records to the operating system without closing the
// file, so a long capture does not lose its tail if the process is killed.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf == nil {
		return nil
	}

	err := w.buf.Flush()
	if w.gz != nil {
		err = multierr.Append(err, w.gz.Flush())
	}
	return err
}

// Close flushes and closes the current file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeCurrent()
}
