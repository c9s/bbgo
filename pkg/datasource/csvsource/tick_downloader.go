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

func Download(path, symbol string, exchange types.ExchangeName, since, until time.Time) (err error) {
	for {
		var (
			subDir   = fmt.Sprintf("%s/%s/%s", path, exchange.String(), symbol)
			fileName = fmt.Sprintf("%s%s.csv", symbol, since.Format("2006-01-02"))
		)

		if fileExists(filepath.Join(subDir, fileName)) {
			since = since.AddDate(0, 0, 1)
			continue
		}

		var url, err = buildURL(exchange, symbol, fileName, since)
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

		err = write(csvContent, subDir, fileName)
		if err != nil {
			log.Error(err)
			break
		}

		since = since.AddDate(0, 0, 1)
		if until.After(since) {
			break
		}
	}

	return err
}

func buildURL(exchange types.ExchangeName, symbol string, fileName string, start time.Time) (url string, err error) {
	switch exchange {
	case types.ExchangeBybit:
		url = fmt.Sprintf("https://public.bybit.com/trading/%s/%s.gz",
			symbol,
			fileName)

	case types.ExchangeBinance:
		url = fmt.Sprintf("https://data.binance.vision/data/spot/daily/aggTrades/%s/%s-aggTrades-%s.zip",
			symbol,
			symbol,
			start.Format("2006-01-02"))
	case types.ExchangeOKEx:
		// todo temporary find a better solution
		coins := strings.Split(kucoin.ToLocalSymbol(symbol), "-")
		if len(coins) == 0 {
			err = fmt.Errorf("%s not supported yet for OKEx.. care to fix it? PR's welcome", symbol)
			return
		}
		baseCoin := coins[0]
		quoteCoin := coins[1]
		url = fmt.Sprintf("https://static.okx.com/cdn/okex/traderecords/aggtrades/daily/%s/%s-%s-aggtrades-%s.zip",
			start.Format("20060102"),
			baseCoin,
			quoteCoin,
			start.Format("2006-01-02"))
	default:
		err = fmt.Errorf("%s not supported yet as csv data source.. care to fix it? PR's welcome", exchange.String())
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

func write(content []byte, saveToPath, fileName string) error {

	if _, err := os.Stat(saveToPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(saveToPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("mkdir %s: %w", saveToPath, err)
		}
	}

	err := os.WriteFile(fmt.Sprintf("%s/%s", saveToPath, fileName), content, 0666)
	if err != nil {
		return fmt.Errorf("write %s: %w", saveToPath+fileName, err)
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
