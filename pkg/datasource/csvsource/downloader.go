package csvsource

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/kucoin"
	"github.com/c9s/bbgo/pkg/types"
)

type MarketType string
type DataType string

const (
	SPOT      MarketType = "spot"
	FUTURES   MarketType = "futures"
	TRADES    DataType   = "trades"
	AGGTRADES DataType   = "aggTrades"
	// todo could be extended to:

	// LEVEL2 = 2
	// https://data.binance.vision/data/futures/um/daily/bookTicker/ADAUSDT/ADAUSDT-bookTicker-2023-11-19.zip
	// update_id	best_bid_price	best_bid_qty	best_ask_price	best_ask_qty	transaction_time	event_time
	// 3.52214E+12	0.3772	        1632	        0.3773	        67521	        1.70035E+12	        1.70035E+12

	// METRICS = 3
	// https://data.binance.vision/data/futures/um/daily/metrics/ADAUSDT/ADAUSDT-metrics-2023-11-19.zip
	// 	create_time	        symbol	sum_open_interest	sum_open_interest_value	count_toptrader_long_short_ratio	sum_toptrader_long_short_ratio	count_long_short_ratio	sum_taker_long_short_vol_ratio
	//  19/11/2023 00:00	ADAUSDT	141979878.00000000	53563193.89339590	    2.33412322	                        1.21401178	                    2.46604727	            0.55265805

	// KLINES    DataType = 4
	// https://public.bybit.com/kline_for_metatrader4/BNBUSDT/2021/BNBUSDT_15_2021-07-01_2021-07-31.csv.gz
	// only few symbols but supported interval options 1m/ 5m/ 15m/ 30m/ 60m/ and only monthly

	// https://data.binance.vision/data/futures/um/daily/klines/1INCHBTC/30m/1INCHBTC-30m-2023-11-18.zip
	// supported interval options 1s/ 1m/ 3m/ 5m/ 15m/ 30m/ 1h/ 2h/ 4h/ 6h/ 8h/ 12h/ 1d/ daily or monthly futures

	// this might be useful for backtesting against mark or index price
	// especially index price can be used across exchanges
	// https://data.binance.vision/data/futures/um/daily/indexPriceKlines/ADAUSDT/1h/ADAUSDT-1h-2023-11-19.zip
	// https://data.binance.vision/data/futures/um/daily/markPriceKlines/ADAUSDT/1h/ADAUSDT-1h-2023-11-19.zip

	// OKex or Bybit do not support direct kLine, metrics or level2 csv download

)

func Download(
	path, symbol string,
	exchange types.ExchangeName,
	market MarketType,
	granularity DataType,
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

		var url, err = buildURL(exchange, symbol, market, granularity, since)
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
		// normalize csv content for trades data
		if granularity == TRADES || granularity == AGGTRADES {
			switch exchange {
			case types.ExchangeOKEx:
				lines, err := parseCsvTicksOkex(csvContent)
				if err != nil {
					return err
				}
				csvContent = []byte(strings.Join(lines, "\n"))
			case types.ExchangeBinance:
				lines, err := parseCsvTicksBinance(csvContent)
				if err != nil {
					return err
				}
				csvContent = []byte(strings.Join(lines, "\n"))
			case types.ExchangeBybit:
				lines, err := parseCsvTicksBybit(csvContent)
				if err != nil {
					return err
				}
				csvContent = []byte(strings.Join(lines, "\n"))
			}
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

// WriteKLines writes csv to path.
func WriteKLines(path, symbol string, klines []types.KLine) (err error) {
	if len(klines) == 0 {
		return fmt.Errorf("no klines to write")
	}
	from := klines[0].StartTime.Time()
	end := klines[len(klines)-1].EndTime.Time()
	to := ""
	if from.Day() != end.Day() {
		to = ":" + end.Format("2006-01-02")
	}

	path = fmt.Sprintf("%s/klines/%s",
		path,
		klines[0].Interval.String(),
	)

	fileName := fmt.Sprintf("%s/%s_%s%s.csv",
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
		return fmt.Errorf("failed to open file: %w", err)
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
			return fmt.Errorf("writing record to file: %w", err)
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func buildURL(
	exchange types.ExchangeName,
	symbol string,
	market MarketType,
	granularity DataType,
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
		if market == FUTURES {
			marketType = "futures/um"
		}
		dataType := "aggTrades"
		if granularity == TRADES {
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
		//TODO: temporary find a better solution ?!
		coins := strings.Split(kucoin.ToLocalSymbol(symbol), "-")
		if len(coins) == 0 {
			err = fmt.Errorf("%s not supported yet for OKEx.. care to fix it? PR's welcome ;)", symbol)
			return
		}
		baseCoin := coins[0]
		quoteCoin := coins[1]
		marketType := "" // for spot market
		if market == FUTURES {
			marketType = "-SWAP"
		}
		dataType := "aggtrades"
		if granularity == TRADES {
			dataType = "trades"
		}
		url = fmt.Sprintf("https://static.okx.com/cdn/okex/traderecords/%s/daily/%s/%s-%s%s-%s-%s.zip",
			dataType,
			start.Format("20060102"),
			baseCoin,
			quoteCoin,
			marketType,
			dataType,
			start.Format(time.DateOnly))
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

// parseCsvTicksOkex parses OKEx trades CSV content and normalizes to: trade_id,side,size,price,timestamp
// OKEx format (has header):
//
//	trade_id, side, size, price, created_time
func parseCsvTicksOkex(csvContent []byte) ([]string, error) {
	rawLines := strings.Split(string(csvContent), "\n")
	if len(rawLines) < 2 {
		return nil, fmt.Errorf("not enough lines in csv content")
	}
	// skip the header line
	rawLines = rawLines[1:]
	var lines []string
	for lineNo, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			return nil, fmt.Errorf("Okex invalid line at %d: %s", lineNo, line)
		}
		tradeID, side, size, price, timestamp := fields[0], fields[1], fields[2], fields[3], fields[4]
		if side != "buy" && side != "sell" {
			return nil, fmt.Errorf("Okex invalid side at line %d: %s", lineNo, side)
		}
		if _, err := strconv.ParseFloat(price, 64); err != nil {
			return nil, fmt.Errorf("Okex invalid price at line %d: %s", lineNo, price)
		}
		if _, err := strconv.ParseFloat(size, 64); err != nil {
			return nil, fmt.Errorf("Okex invalid size at line %d: %s", lineNo, size)
		}
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			return nil, fmt.Errorf("Okex invalid timestamp at line %d: %s", lineNo, timestamp)
		}
		lines = append(lines, strings.Join([]string{tradeID, side, size, price, timestamp}, ","))
	}
	return lines, nil
}

// parseCsvTicksBinance parses Binance aggTrades/trades CSV content and normalizes to: trade_id,side,size,price,timestamp
// Binance aggTrades format (no header):
//
//	trade_id, price, quantity, first_trade_id, last_trade_id, timestamp, is_buyer_maker, is_best_match
func parseCsvTicksBinance(csvContent []byte) ([]string, error) {
	rawLines := strings.Split(string(csvContent), "\n")
	var lines []string
	for lineNo, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 7 {
			return nil, fmt.Errorf("Binance invalid line at %d: %s", lineNo, line)
		}
		tradeID, price, quantity, timestamp, isBuyerMakerStr := fields[0], fields[1], fields[2], fields[5], fields[6]
		if _, err := strconv.ParseFloat(price, 64); err != nil {
			return nil, fmt.Errorf("Binance invalid price at line %d: %s", lineNo, price)
		}
		if _, err := strconv.ParseFloat(quantity, 64); err != nil {
			return nil, fmt.Errorf("Binance invalid quantity at line %d: %s", lineNo, quantity)
		}
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			return nil, fmt.Errorf("Binance invalid timestamp at line %d: %s", lineNo, timestamp)
		}
		isBuyerMaker, err := strconv.ParseBool(isBuyerMakerStr)
		if err != nil {
			return nil, fmt.Errorf("Binance invalid is_buyer_maker at line %d: %s", lineNo, isBuyerMakerStr)
		}
		side := "buy"
		if isBuyerMaker {
			side = "sell"
		}
		lines = append(lines, strings.Join([]string{tradeID, side, quantity, price, timestamp}, ","))
	}
	return lines, nil
}

// parseCsvTicksBybit parses Bybit trades CSV content and normalizes to: trade_id,side,size,price,timestamp
// Bybit format (has header):
//
//	timestamp, symbol, side, size, price, tickDirection, trdMatchID, grossValue, homeNotional, foreignNotional
func parseCsvTicksBybit(csvContent []byte) ([]string, error) {
	rawLines := strings.Split(string(csvContent), "\n")
	if len(rawLines) < 2 {
		return nil, fmt.Errorf("not enough lines in csv content")
	}
	var lines []string
	// skip the header line
	for lineNo, line := range rawLines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			return nil, fmt.Errorf("Bybit invalid line at %d: %s", lineNo, line)
		}
		timestampStr, side, size, price := fields[0], fields[2], fields[3], fields[4]
		if side != "Buy" && side != "Sell" {
			return nil, fmt.Errorf("Bybit invalid side at line %d: %s", lineNo, side)
		}
		if _, err := strconv.ParseFloat(price, 64); err != nil {
			return nil, fmt.Errorf("Bybit invalid price at line %d: %s", lineNo, price)
		}
		if _, err := strconv.ParseFloat(size, 64); err != nil {
			return nil, fmt.Errorf("Bybit invalid size at line %d: %s", lineNo, size)
		}
		ts, err := strconv.ParseFloat(timestampStr, 64)
		if err != nil {
			return nil, fmt.Errorf("Bybit invalid timestamp at line %d: %s", lineNo, timestampStr)
		}
		// convert fractional seconds to milliseconds
		timestampMs := strconv.FormatInt(int64(ts*1000), 10)
		// use line index as trade id (bybit csv has no trade id)
		tradeID := strconv.Itoa(lineNo + 1)
		lines = append(lines, strings.Join([]string{tradeID, strings.ToLower(side), size, price, timestampMs}, ","))
	}
	return lines, nil
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
