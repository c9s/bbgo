package csvsource

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// CSVTickDecoder is an extension point for CSVTickReader to support custom file formats.
type CSVTickDecoder func(record []string, index int) (*CsvTick, error)

// NewBinanceCSVTickReader creates a new CSVTickReader for Binance CSV files.
func NewBinanceCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BinanceCSVTickDecoder,
	}
}

// BinanceCSVKLineDecoder decodes a CSV record from Binance into a CsvTick.
func BinanceCSVTickDecoder(row []string, _ int) (*CsvTick, error) {
	if len(row) < 7 {
		return nil, ErrNotEnoughColumns
	}
	// example csv row
	// id, price, qty, base_qty, time, is_buyer_maker,
	// 11782578,6.00000000,1.00000000,14974844,14974844,1698623884463,True
	qty := fixedpoint.MustNewFromString(row[2])
	baseQty := fixedpoint.MustNewFromString(row[2])
	price := fixedpoint.MustNewFromString(row[1])
	// isBuyerMaker=false trade will qualify as BUY.
	id, err := strconv.ParseUint(row[0], 10, 64)
	if err != nil {
		return nil, err
	}
	isBuyerMaker, err := strconv.ParseBool(row[5])
	if err != nil {
		return nil, err
	}
	side := types.SideTypeBuy
	if isBuyerMaker {
		side = types.SideTypeSell
	}
	return &CsvTick{
		TradeID:         id,
		Exchange:        types.ExchangeBinance,
		Side:            side,
		Size:            qty,
		Price:           price,
		HomeNotional:    price.Mul(qty),
		ForeignNotional: price.Mul(baseQty),
		Timestamp:       types.MustParseMillisecondTimestamp(row[5]),
	}, nil
}

// NewBinanceCSVTickReader creates a new CSVTickReader for Bybit CSV files.
func NewBybitCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BybitCSVTickDecoder,
	}
}

// BybitCSVTickDecoder decodes a CSV record from Bybit into a CsvTick.
func BybitCSVTickDecoder(row []string, index int) (*CsvTick, error) {
	// example csv row
	// timestamp,symbol,side,size,price,tickDirection,trdMatchID,grossValue,homeNotional,foreignNotional
	// 1649054912,FXSUSDT,Buy,0.01,38.32,PlusTick,9c30abaf-80ae-5ebf-9850-58fe7ed4bac8,3.832e+07,0.01,0.3832
	if len(row) < 9 {
		return nil, ErrNotEnoughColumns
	}
	if index == 0 {
		return nil, nil
	}
	side, err := types.StrToSideType(row[2])
	if err != nil {
		return nil, ErrInvalidOrderSideFormat
	}
	id, err := uuidStringToUInt(row[6])
	if err != nil {
		return nil, ErrInvalidOrderSideFormat
	}
	return &CsvTick{
		TradeID:         id,
		Symbol:          row[1],
		Exchange:        types.ExchangeBybit,
		Side:            side,
		Size:            fixedpoint.MustNewFromString(row[3]),
		Price:           fixedpoint.MustNewFromString(row[4]),
		HomeNotional:    fixedpoint.MustNewFromString(row[8]),
		ForeignNotional: fixedpoint.MustNewFromString(row[9]),
		TickDirection:   row[5], // todo does this seem promising to define for other exchanges too?
		Timestamp:       types.MustParseMillisecondTimestamp(row[0]),
	}, nil
}

func uuidStringToUInt(uuidStr string) (uint64, error) {
	// Remove hyphens from the UUID string
	uuidStr = strings.Replace(uuidStr, "-", "", -1)

	// Parse the hexadecimal string into a big integer
	uuidBigInt, success := new(big.Int).SetString(uuidStr, 16)
	if !success {
		return 0, fmt.Errorf("Failed to parse UUID as a big integer")
	}

	return uuidBigInt.Uint64(), nil
}
