package archive

import (
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/multierr"
)

// Entry is a single decompressed member of an archive. Closing it releases the
// decompressor and the underlying file.
type Entry struct {
	// Name is the member name inside the archive, or the file name for a
	// plain or gzipped file.
	Name string

	r       io.Reader
	closers []io.Closer
}

func (e *Entry) Read(p []byte) (int, error) { return e.r.Read(p) }

func (e *Entry) Close() error {
	var err error
	// close in reverse order: decompressor before the file it reads from
	for i := len(e.closers) - 1; i >= 0; i-- {
		err = multierr.Append(err, e.closers[i].Close())
	}
	e.closers = nil
	return err
}

// Open opens a local archive for streaming and returns its single data member.
//
// It handles the three shapes exchanges publish: a .zip holding one CSV
// (Binance, OKX), a .csv.gz (Bybit), and a plain .csv. The content is streamed
// rather than read into memory, because one day of aggregated trades for a
// liquid symbol is hundreds of megabytes uncompressed.
func Open(path string) (*Entry, error) {
	switch {
	case strings.HasSuffix(path, ".zip"):
		return openZip(path)
	case strings.HasSuffix(path, ".gz"):
		return openGzip(path)
	default:
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &Entry{Name: filepath.Base(path), r: f, closers: []io.Closer{f}}, nil
	}
}

func openZip(path string) (*Entry, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("archive: opening zip %s: %w", path, err)
	}

	var member *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(filepath.Base(f.Name), ".") {
			continue
		}
		if member != nil {
			zr.Close()
			return nil, fmt.Errorf("archive: zip %s has more than one member (%s, %s)",
				path, member.Name, f.Name)
		}
		member = f
	}

	if member == nil {
		zr.Close()
		return nil, fmt.Errorf("archive: zip %s is empty", path)
	}

	rc, err := member.Open()
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("archive: opening %s in %s: %w", member.Name, path, err)
	}

	return &Entry{
		Name:    member.Name,
		r:       rc,
		closers: []io.Closer{zr, rc},
	}, nil
}

func openGzip(path string) (*Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("archive: opening gzip %s: %w", path, err)
	}

	return &Entry{
		Name:    strings.TrimSuffix(filepath.Base(path), ".gz"),
		r:       gr,
		closers: []io.Closer{f, gr},
	}, nil
}

// OpenCSV opens an archive and wraps its member in a CSV Reader.
func OpenCSV(path string) (*Reader, io.Closer, error) {
	entry, err := Open(path)
	if err != nil {
		return nil, nil, err
	}
	return NewReader(entry), entry, nil
}
