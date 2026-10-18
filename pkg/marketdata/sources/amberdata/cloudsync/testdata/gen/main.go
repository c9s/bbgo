//go:build ignore

// Command gen builds the Parquet fixture from AmberData's public sample.
//
// The sample is a real Binance USDⓈ-M order book updates file and is 40 MB, far
// too large to commit, so this keeps the first few hundred rows. It is run by
// hand, never by CI: the point of committing it is that the fixture's provenance
// stays reproducible, not that it is regenerated automatically.
//
// Usage:
//
//	go run ./testdata/gen -rows 400
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// sampleURL is publicly readable with no credentials, unlike the CloudSync
// bucket itself, which is Requester Pays.
const sampleURL = "https://amberdata-samples.s3.amazonaws.com/market/futures/order-book-updates/2026-02-15.binance.BTCUSDT.00.parquet"

const outputName = "order-book-updates-binance-BTCUSDT-trimmed.parquet"

type metadataRow struct {
	FirstUpdateID int64  `parquet:"firstUpdateId,optional"`
	Version       int64  `parquet:"version,optional"`
	LastID        int64  `parquet:"lastId,optional"`
	Mrid          string `parquet:"mrid,optional"`
	ID            string `parquet:"id,optional"`
}

type bookRow struct {
	Exchange                     string      `parquet:"exchange,optional"`
	Instrument                   string      `parquet:"instrument,optional"`
	ExchangeTimestamp            int64       `parquet:"exchangeTimestamp,optional"`
	ExchangeTimestampNanoseconds int64       `parquet:"exchangeTimestampNanoseconds,optional"`
	IsBid                        bool        `parquet:"isBid,optional"`
	ReceivedTimestamp            int64       `parquet:"receivedTimestamp,optional"`
	ReceivedTimestampNanoseconds int64       `parquet:"receivedTimestampNanoseconds,optional"`
	Timestamp                    int64       `parquet:"timestamp,optional"`
	Metadata                     metadataRow `parquet:"metadata,optional"`
	Sequence                     int64       `parquet:"sequence,optional"`
	Data                         [][]float64 `parquet:"data,list,optional"`
	Status                       string      `parquet:"status,optional"`
}

func main() {
	rows := flag.Int("rows", 400, "how many rows to keep")
	levels := flag.Int("levels", 12, "how many price levels to keep per row")
	cache := flag.String("cache", "", "reuse an already-downloaded sample instead of fetching it")
	flag.Parse()

	path := *cache
	if path == "" {
		var err error
		path, err = download()
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(path)
	}

	all, err := parquet.ReadFile[bookRow](path)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("read %d rows from the sample", len(all))

	kept := all
	if len(kept) > *rows {
		kept = kept[:*rows]
	}

	// Truncating depth keeps the fixture small. The levels that remain are the
	// venue's own values, unmodified.
	for i := range kept {
		if len(kept[i].Data) > *levels {
			kept[i].Data = kept[i].Data[:*levels]
		}
	}

	out := filepath.Join("testdata", outputName)
	if err := parquet.WriteFile(out, kept); err != nil {
		log.Fatal(err)
	}

	st, err := os.Stat(out)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d rows, %d bytes\n", out, len(kept), st.Size())
}

func download() (string, error) {
	log.Printf("downloading %s", sampleURL)

	resp, err := http.Get(sampleURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gen: unexpected status %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "amberdata-sample-*.parquet")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}

	return f.Name(), nil
}
