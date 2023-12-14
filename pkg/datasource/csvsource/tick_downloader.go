package csvsource

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/types"
)

func Download(
	path, symbol string,
	exchange types.ExchangeName,
	market types.MarketType,
	granularity types.MarketDataType,
	since, until time.Time,
) (err error) {
	for {
		var (
			fileName = fmt.Sprintf("%s-%s.csv", symbol, since.Format("2006-01-02"))
		)

		if fileExists(filepath.Join(path, fileName)) {
			since = since.AddDate(0, 0, 1)
			continue
		}

		var url, err = buildURL(exchange, symbol, market, granularity, fileName, since)
		if err != nil {
			log.Error(err)
			break
		}

		log.Info("fetching ", url)

		csvContent, err := readCSVFromUrl(exchange, url)
		if err != nil {
			log.Error(err)
			break
		}

		err = write(csvContent, fmt.Sprintf("%s/%s", path, granularity), fileName)
		if err != nil {
			log.Error(err)
			break
		}

		since = since.AddDate(0, 0, 1)
		if since.After(until) {
			break
		}
	}

	return err
}

func buildURL(
	exchange types.ExchangeName,
	symbol string,
	market types.MarketType,
	granularity types.MarketDataType,
	fileName string,
	start time.Time,
) (url string, err error) {
	switch exchange {
	case types.ExchangeBybit:
		// bybit doesn't seem to differentiate between spot and futures market or trade type in their csv dumps ;(
		url = fmt.Sprintf("https://public.bybit.com/trading/%s/%s%s.csv.gz",
			symbol,
			symbol,
			start.Format("2006-01-02"),
		)
	case types.ExchangeBinance:
		marketType := "spot"
		if market == types.MarketTypeFutures {
			marketType = "futures/um"
		}
		dataType := "aggTrades"
		if granularity == types.MarketDataTypeTrades {
			dataType = "trades"
		}
		url = fmt.Sprintf("https://data.binance.vision/data/%s/daily/%s/%s/%s-%s-%s.zip",
			marketType,
			dataType,
			symbol,
			symbol,
			dataType,
			start.Format("2006-01-02"))

	case types.ExchangeOKEx:
		// todo temporary find a better solution ?!
		coins := strings.Split(kucoin.ToLocalSymbol(symbol), "-")
		if len(coins) == 0 {
			err = fmt.Errorf("%s not supported yet for OKEx.. care to fix it? PR's welcome ;)", symbol)
			return
		}
		baseCoin := coins[0]
		quoteCoin := coins[1]
		marketType := "" // for spot market
		if market == types.MarketTypeFutures {
			marketType = "-SWAP"
		}
		dataType := "aggtrades"
		if granularity == types.MarketDataTypeTrades {
			dataType = "trades"
		}
		url = fmt.Sprintf("https://static.okx.com/cdn/okex/traderecords/%s/daily/%s/%s-%s%s-%s-%s.zip",
			dataType,
			start.Format("20060102"),
			baseCoin,
			quoteCoin,
			marketType,
			dataType,
			start.Format("2006-01-02"))
	default:
		err = fmt.Errorf("%s not supported yet as csv data source.. care to fix it? PR's welcome ;)", exchange.String())
	}

	return url, err
}

func readCSVFromUrl(exchange types.ExchangeName, url string) (csvContent []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get error, url %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %w", err)
	}

	switch exchange {
	case types.ExchangeBybit:
		csvContent, err = gunzip(body)
		if err != nil {
			return nil, fmt.Errorf("gunzip data %s: %w", exchange, err)
		}

	case types.ExchangeBinance:
		csvContent, err = unzip(body)
		if err != nil {
			return nil, fmt.Errorf("unzip data %s: %w", exchange, err)
		}

	case types.ExchangeOKEx:
		csvContent, err = unzip(body)
		if err != nil {
			return nil, fmt.Errorf("unzip data %s: %w", exchange, err)
		}
	}

	return csvContent, nil
}

func write(content []byte, path, fileName string) error {

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
	}

	dest := filepath.Join(path, fileName)

	err := os.WriteFile(dest, content, 0666)
	if err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	return nil
}

func unzip(data []byte) (resData []byte, err error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		log.Error(err)
	}

	if zipReader == nil || len(zipReader.File) == 0 {
		return nil, errors.New("no data to unzip")
	}

	// Read all the files from zip archive
	for _, zipFile := range zipReader.File {
		resData, err = readZipFile(zipFile)
		if err != nil {
			log.Error(err)
			break
		}
	}

	return
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func gunzip(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}

func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
