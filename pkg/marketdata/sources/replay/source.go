package replay

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// Config configures a replay Source.
type Config struct {
	// Path is a recording file or a directory of them.
	Path string `json:"path" yaml:"path"`

	// Name overrides the source name in logs and merge diagnostics.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Symbols restricts replay to these symbols. Empty means all.
	Symbols []string `json:"symbols,omitempty" yaml:"symbols,omitempty"`
}

// Source replays recorded market data.
type Source struct {
	cfg   Config
	files []string

	// capability is derived once by scanning the recordings' headers, so that a
	// merge can tell whether the order book is actually available before it
	// starts reading gigabytes.
	capability marketdata.Capability
}

var _ marketdata.Source = (*Source)(nil)

// New scans Path and returns a Source over the recordings it finds.
func New(cfg Config) (*Source, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("replay: path is required")
	}

	files, err := findRecordings(cfg.Path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("replay: no recordings found under %s", cfg.Path)
	}

	s := &Source{cfg: cfg, files: files}
	if err := s.scanHeaders(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Source) Name() string {
	if s.cfg.Name != "" {
		return s.cfg.Name
	}
	return "replay/" + filepath.Base(strings.TrimRight(s.cfg.Path, string(filepath.Separator)))
}

func (s *Source) Capabilities() marketdata.Capability { return s.capability }

// Files returns the recordings this source will read, in order.
func (s *Source) Files() []string { return slices.Clone(s.files) }

// scanHeaders reads only the first line of each recording to learn what it
// holds, which is cheap and avoids opening a request only to find it empty.
func (s *Source) scanHeaders() error {
	s.capability = marketdata.Capability{HasHistory: true}

	for _, path := range s.files {
		header, closer, _, err := openRecording(path)
		if err != nil {
			return err
		}
		closer.Close()

		if header.Version != FormatVersion {
			return fmt.Errorf("replay: %s has format version %d, this build understands %d",
				filepath.Base(path), header.Version, FormatVersion)
		}

		if header.Exchange != "" && !slices.Contains(s.capability.Exchanges, header.Exchange) {
			s.capability.Exchanges = append(s.capability.Exchanges, header.Exchange)
		}
		for _, sym := range header.Symbols {
			if !slices.Contains(s.capability.Symbols, sym) {
				s.capability.Symbols = append(s.capability.Symbols, sym)
			}
		}
		for _, ch := range header.Channels {
			if !slices.Contains(s.capability.Channels, ch) {
				s.capability.Channels = append(s.capability.Channels, ch)
			}
		}
	}

	// A recording written without an explicit channel list is assumed to carry
	// whatever the recorder binds, rather than nothing.
	if len(s.capability.Channels) == 0 {
		s.capability.Channels = []types.Channel{
			types.BookChannel,
			types.MarketTradeChannel,
			types.AggTradeChannel,
			types.BookTickerChannel,
			types.KLineChannel,
		}
	}

	if len(s.cfg.Symbols) > 0 {
		s.capability.Symbols = slices.Clone(s.cfg.Symbols)
	}

	return nil
}

func (s *Source) Open(ctx context.Context, req marketdata.Request) (marketdata.Cursor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.capability.Validate(s.Name(), req); err != nil {
		return nil, err
	}

	symbols := s.cfg.Symbols
	if len(symbols) == 0 {
		symbols = req.Symbols()
	}

	channels := make([]types.Channel, 0, len(req.Subscriptions))
	for _, sub := range req.Subscriptions {
		if !slices.Contains(channels, sub.Channel) {
			channels = append(channels, sub.Channel)
		}
	}

	return &cursor{
		ctx:      ctx,
		files:    s.files,
		req:      req,
		symbols:  symbols,
		channels: channels,
		fileIdx:  -1,
		logger:   log.WithField("component", s.Name()),
	}, nil
}

// findRecordings returns the recordings under path, sorted by name. The file
// names embed their UTC hour, so sorting by name sorts by time.
func findRecordings(path string) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !st.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".jsonl") || strings.HasSuffix(p, FileExtension) {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// openRecording opens a recording and consumes its header line.
func openRecording(path string) (Header, io.Closer, *json.Decoder, error) {
	var header Header

	f, err := os.Open(path)
	if err != nil {
		return header, nil, nil, err
	}

	var r io.Reader = f
	closer := io.Closer(f)

	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return header, nil, nil, fmt.Errorf("replay: opening %s: %w", filepath.Base(path), err)
		}
		r = gz
		closer = closerFunc(func() error {
			gzErr := gz.Close()
			if fErr := f.Close(); fErr != nil {
				return fErr
			}
			return gzErr
		})
	}

	dec := json.NewDecoder(bufio.NewReaderSize(r, 64*1024))
	if err := dec.Decode(&header); err != nil {
		closer.Close()
		return header, nil, nil, fmt.Errorf("replay: reading header of %s: %w",
			filepath.Base(path), err)
	}

	return header, closer, dec, nil
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
