package mexc

import (
	"golang.org/x/time/rate"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MX = "MX"

var queryLimiter = rate.NewLimiter(rate.Every(time.Second), 20)
var debug bool = false

var urlTemplate url.URL = url.URL{
	Scheme:  "https",
	Host:    "api.mexc.com",
	Path:    "/api/v3",
	RawPath: "/api/v3",
}

var log = logrus.WithField("exchange", "mexc")

type Exchange struct {
	key, secret string
	client      *http.Client
}

func New(key, secret string) *Exchange {
	return &Exchange{key, secret, nil}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeMEXC
}

func (e *Exchange) PlatformFeeCurrency() string {
	return MX
}

func (e *Exchange) publicRequest(ctx context.Context, method string, path string, params url.Values, result interface{}) error {
	if err := queryLimiter.Wait(ctx); err != nil {
		return err
	}
	u := urlTemplate
	u.Path += path
	u.RawPath += path
	u.RawQuery = params.Encode()
	log.Println(u.String())
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return errors.New(fmt.Sprintf("return status value: %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

func (e *Exchange) signRequest(ctx context.Context, method string, path string, params url.Values, result interface{}) error {
	timestamp, err := e.time(ctx)
	if err != nil {
		return err
	}
	if err := queryLimiter.Wait(ctx); err != nil {
		return err
	}
	// user time might not be synchronized with the server
	// timestamp := time.Now().UnixMilli()
	params.Add("timestamp", strconv.FormatInt(timestamp, 10))
	queryString := params.Encode()
	sign := hmac.New(sha256.New, []byte(e.secret))
	sign.Write([]byte(queryString))
	signature := fmt.Sprintf("%x", sign.Sum([]byte{}))
	params.Add("signature", signature)
	u := urlTemplate
	u.Path += path
	u.RawPath += path
	u.RawQuery = params.Encode()
	log.Infof("%s %s, %s", u.String(), method, queryString)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return errors.New(fmt.Sprintf("return status value: %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if debug {
		log.Infof("%s", data)
	}
	return json.Unmarshal(data, result)
}

type pingResp struct{}

func (e *Exchange) ping(ctx context.Context) bool {
	err := e.publicRequest(ctx, "GET", "/ping", url.Values{}, &pingResp{})
	if err != nil {
		log.WithError(err).Errorf("ping error")
		return false
	}
	return true
}

type Time struct {
	ServerTime int64 `json:"serverTime"`
}

func (e *Exchange) time(ctx context.Context) (int64, error) {
	var t Time
	if err := e.publicRequest(ctx, "GET", "/time", url.Values{}, &t); err != nil {
		log.WithError(err).Errorf("time error")
		return 0, err
	}
	return t.ServerTime, nil
}

type ticker24hr struct {
	CloseTime   int64            `json:"closeTime"`
	OpenTime    int64            `json:"openTime"`
	Symbol      string           `json:"symbol"`
	QuoteVolume fixedpoint.Value `json:"quoteVolume"`
	Volume      fixedpoint.Value `json:"volume"`
	LowPrice    fixedpoint.Value `json:"lowPrice"`
	HighPrice   fixedpoint.Value `json:"highPrice"`
	OpenPrice   fixedpoint.Value `json:"openPrice"`
	AskPrice    fixedpoint.Value `json:"askPrice"`
	BidPrice    fixedpoint.Value `json:"bidPrice"`
	LastPrice   fixedpoint.Value `json:"lastPrice"`
}

func (t *ticker24hr) ToTicker() types.Ticker {
	return types.Ticker{
		Time:   time.UnixMilli(t.OpenTime),
		Volume: t.Volume,
		Last:   t.LastPrice,
		Open:   t.OpenPrice,
		High:   t.HighPrice,
		Low:    t.LowPrice,
		Buy:    t.BidPrice,
		Sell:   t.AskPrice,
	}
}

/*{"symbol":"APEUSDT","priceChange":"0.0852","priceChangePercent":"0.0099905","prevClosePrice":"8.5281","lastPrice":"8.6133","lastQty":"","bidPrice":"8.5983","bidQty":"","askPrice":"8.6242","askQty":"","openPrice":"8.5281","highPrice":"9.2478","lowPrice":"8.14","volume":"166512.31","quoteVolume":null,"openTime":1652869200000,"closeTime":1652869439540,"count":null}*/

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	v := url.Values{}
	v.Add("symbol", symbol)
	t := ticker24hr{}
	if err := e.publicRequest(ctx, "GET", "/ticker/24hr", v, &t); err != nil {
		log.WithError(err).Errorf("queryTicker error")
		return nil, err
	}
	result := t.ToTicker()
	return &result, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (result map[string]types.Ticker, err error) {
	result = make(map[string]types.Ticker)
	// return all symbols if given empty symbols
	if len(symbols) == 0 {
		t := []ticker24hr{}
		if err := e.publicRequest(ctx, "GET", "/ticker/24hr", url.Values{}, &t); err != nil {
			log.WithError(err).Errorf("queryTicker error")
			return result, err
		}
		for _, tt := range t {
			result[tt.Symbol] = tt.ToTicker()
		}
		return result, nil
	}
	// Otherwise query one by one.
	for _, s := range symbols {
		ticker, err := e.QueryTicker(ctx, s)
		if err != nil {
			return result, err
		}
		result[s] = *ticker
	}
	return result, nil
}

type symbolInfo struct {
	Symbol                     string   `json:"symbol"`
	Status                     string   `json:"status"`
	BaseAsset                  string   `json:"baseAsset"`
	BaseAssetPrecision         int      `json:"baseAssetPrecision"`
	QuoteAsset                 string   `json:"quoteAsset"`
	QuotePrecision             int      `json:"quotePrecision"`
	QuoteAssetPrecision        int      `json:"quoteAssetPrecision"`
	BaseCommissionPrecision    int      `json:"baseCommissionPrecision"`
	QuoteCommissionPrecision   int      `json:"quoteCommissionPrecision"`
	OrderTypes                 []string `json:"orderTypes"`
	IcebergAllowed             bool     `json:"icebergAllowed"`
	OcoAllowed                 bool     `json:"ocoAllowed"`
	QuoteOrderQtyMarketAllowed bool     `json:"QuoteOrderQtyMarketAllowed"`
	SpotTradingAllowed         bool     `json:"isSpotTradingAllowed"`
	MarginTradingAllowed       bool     `json:"isMarginTradingAllowed"`
	Permissions                []string `json:"permissions"`
}

func (s *symbolInfo) ToMarket() types.Market {
	return types.Market{
		Symbol:          s.Symbol,
		LocalSymbol:     s.Symbol,
		PricePrecision:  s.QuotePrecision, // Or quoteAssetPrecision?
		VolumePrecision: s.BaseAssetPrecision,
		QuoteCurrency:   s.QuoteAsset,
		BaseCurrency:    s.BaseAsset,
		MinNotional:     fixedpoint.NewFromInt(5),
	}
}

type exchangeInfo struct {
	Symbols []symbolInfo `json:"symbols"`
}

/*{"timezone":"CST","serverTime":1652948144443,"rateLimits":[],"exchangeFilters":[],"symbols":[{"symbol":"TOMO3LUSDT","status":"ENABLED","baseAsset":"TOMO3L","baseAssetPrecision":2,"quoteAsset":"USDT","quotePrecision":3,"quoteAssetPrecision":3,"baseCommissionPrecision":2,"quoteCommissionPrecision":3,"orderTypes":["LIMIT","LIMIT_MAKER"],"icebergAllowed":false,"ocoAllowed":false,"quoteOrderQtyMarketAllowed":false,"isSpotTradingAllowed":true,"isMarginTradingAllowed":false,"permissions":["SPOT"],"filters":[]},{"symbol":"ALEPHUSDT","status":"ENABLED","baseAsset":"ALEPH","baseAssetPrecision":2,"quoteAsset":"USDT","quotePrecision":4,"quoteAssetPrecision":4,"baseCommissionPrecision":2,"quoteCommissionPrecision":4,"orderTypes":["LIMIT","LIMIT_MAKER"],"icebergAllowed":false,"ocoAllowed":false,"quoteOrderQtyMarketAllowed":false,"isSpotTradingAllowed":true,"isMarginTradingAllowed":false,"permissions":["SPOT"],"filters":[]}]}*/

func (e *Exchange) QueryMarkets(ctx context.Context) (result types.MarketMap, err error) {
	result = make(map[string]types.Market)
	m := exchangeInfo{}
	if err := e.publicRequest(ctx, "GET", "/exchangeInfo", url.Values{}, &m); err != nil {
		log.WithError(err).Errorf("query markets failed")
		return result, err
	}
	for _, mm := range m.Symbols {
		result[mm.Symbol] = mm.ToMarket()
	}
	return result, err
}

type depth struct {
	LastUpdatedId int64                `json:"lastUpdatedId"`
	Bids          [][]fixedpoint.Value `json:"bids"`
	Asks          [][]fixedpoint.Value `json:"asks"`
}

// MEXC doesn't have a way to query open orders' snapshot
// Only has different depths of aggregated orders
func (e *Exchange) QueryMarketOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	v := url.Values{}
	v.Set("symbol", symbol)
	//v.Set("limit", 100)  // default: 100, max: 5000
	d := depth{}
	if err := e.publicRequest(ctx, "GET", "/depth", v, &d); err != nil {
		log.WithError(err).Errorf("query open orders failed")
		return orders, err
	}
	for _, pv := range d.Bids {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:   symbol,
				Side:     types.SideTypeBuy,
				Quantity: pv[1],
				Price:    pv[0],
			},
			Exchange: types.ExchangeMEXC,
		}
		orders = append(orders, order)
	}

	for _, pv := range d.Asks {
		order := types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:   symbol,
				Side:     types.SideTypeSell,
				Quantity: pv[1],
				Price:    pv[0],
			},
			Exchange: types.ExchangeMEXC,
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func FromInterval(in types.Interval) string {
	switch in {
	case types.Interval1m, types.Interval5m, types.Interval15m, types.Interval30m, types.Interval4h /*types.Interval8h,*/, types.Interval1d, types.Interval1M:
		return string(in)
	case types.Interval1h:
		return "60m"
	default:
		log.Errorf("interval not supported: %s", in)
		return ""
	}
}

func (e *Exchange) queryKLinesSafe(ctx context.Context, interval types.Interval, v url.Values) (result []types.KLine, err error) {
	k := [][]json.RawMessage{}
	if err := e.publicRequest(ctx, "GET", "/klines", v, &k); err != nil {
		log.WithError(err).Errorf("query klines failed")
		return result, err
	}
	// last kline will be in the end
	for i := len(k) - 1; i >= 0; i-- {
		kk := k[i]
		kline := types.KLine{
			Exchange:  types.ExchangeMEXC,
			Interval:  interval,
			Symbol:    v["symbol"][0],
			StartTime: types.Time(types.MustParseMillisecondTimestamp(string(kk[0]))),
			EndTime:   types.Time(types.MustParseMillisecondTimestamp(string(kk[6]))),
		}
		kline.Open.UnmarshalJSON(kk[1])
		kline.High.UnmarshalJSON(kk[2])
		kline.Low.UnmarshalJSON(kk[3])
		kline.Close.UnmarshalJSON(kk[4])
		kline.Volume.UnmarshalJSON(kk[5])
		kline.QuoteVolume.UnmarshalJSON(kk[7])

		result = append(result, kline)
	}

	return result, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) (result []types.KLine, err error) {
	iv := FromInterval(interval)
	if len(iv) == 0 {
		return result, err
	}
	var startTime int64 = 0
	var endTime int64 = 0
	if options.StartTime != nil {
		startTime = options.StartTime.UnixMilli()
	}
	if options.EndTime != nil {
		endTime = options.EndTime.UnixMilli()
	}
	if startTime == 0 {
		if endTime == 0 {
			return result, errors.New("should provide at least startTime or endTime")
		}
		if options.Limit == 0 {
			return result, errors.New("should provide at least limit or startTime")
		}
		startTime = endTime - int64(options.Limit)*60000*int64(interval.Minutes())
	}
	if endTime == 0 {
		if options.Limit == 0 {
			return result, errors.New("should provide at least limit or endTime")
		}
		endTime = startTime + int64(options.Limit)*60000*int64(interval.Minutes())
	}
	// if time range too large, query latest options.Limit klines
	if options.Limit > 0 && endTime-startTime > int64(options.Limit)*60000*int64(interval.Minutes()) {
		startTime = endTime - int64(options.Limit)*60000*int64(interval.Minutes())
	} else {
		options.Limit = int((endTime - startTime) / 60000 / int64(interval.Minutes()))
		if options.Limit == 0 {
			log.Infof("hello %v %v", endTime, startTime)
			return result, nil
		}
	}
	// (endTime, startTime]
	if options.Limit <= 1000 {
		v := url.Values{}
		v.Set("symbol", symbol)
		v.Set("interval", iv)
		v.Set("limit", strconv.Itoa(options.Limit))
		v.Set("startTime", strconv.FormatInt(endTime, 10))
		return e.queryKLinesSafe(ctx, interval, v)
	} else {
		diff := int64(interval.Minutes()) * 60000 * 1000
		for i := endTime + diff; i >= startTime; i -= diff {
			v := url.Values{}
			v.Set("symbol", symbol)
			v.Set("interval", iv)
			v.Set("startTime", strconv.FormatInt(i, 10))
			if i-diff > startTime {
				v.Set("limit", "1000")
			} else {
				limit := (i - startTime) / int64(interval.Minutes()) / 60000
				if limit == 0 {
					log.Infof("hello %v %v", i, startTime)
					return result, nil
				}
				v.Set("limit", strconv.FormatInt(limit, 10))
			}
			// MEXC only stores the latest 1000 lines
			return e.queryKLinesSafe(ctx, interval, v)
			/*out, err := e.queryKLinesSafe(ctx, interval, v)
			if err != nil {
				return result, err
			}
			result = append(out, result...)*/
		}
		return result, nil
	}
}

// private

type cancelOrderResp struct {
	Symbol              string            `json:"symbol"`
	OrigClientOrderId   string            `json:"origClientOrderId"`
	OrderId             string            `json:"orderId"`
	ClientOrderId       string            `json:"clientOrderId"`
	Price               fixedpoint.Value  `json:"price"`
	OrigQty             fixedpoint.Value  `json:"origQty"`
	ExecutedQty         fixedpoint.Value  `json:"executedQty"`
	CummulativeQuoteQty fixedpoint.Value  `json:"cummulativeQuoteQty"`
	Status              types.OrderStatus `json:"status"`
	TimeInForce         types.TimeInForce `json:"timeInForce"`
	Type                types.OrderType   `json:"type"`
	Side                types.SideType    `json:"side"`
}

func (c *cancelOrderResp) ToOrder() types.Order {
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: c.ClientOrderId,
			Symbol:        c.Symbol,
			Side:          c.Side,
			Type:          c.Type,
			Quantity:      c.OrigQty,
			Price:         c.Price,
			TimeInForce:   c.TimeInForce,
			IsFutures:     false,
		},
		Status:           c.Status,
		ExecutedQuantity: c.ExecutedQty,
	}
}
func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err error) {
	for _, o := range orders {
		v := url.Values{}
		v.Set("symbol", o.Symbol)
		v.Set("orderId", strconv.FormatUint(o.OrderID, 10))
		if len(o.UUID) > 0 {
			v.Set("origClientOrderId", o.UUID)
			v.Set("newClientOrderId", o.UUID)
		}
		var result cancelOrderResp
		if err = e.signRequest(ctx, "DELETE", "/order", v, result); err != nil {
			log.WithError(err).Errorf("cancel failed")
			continue
		}
		if result.Status != types.OrderStatusCanceled &&
			result.Status != types.OrderStatusPartiallyCanceled {
			log.Debugf("cancel failed: OrderId: %s, Symbol: %s, Status: %s",
				result.OrderId, result.Symbol, result.Status)
		} else {
			log.Debugf("cancel succeed: OrderId: %s, Symbol: %s, Status: %s",
				result.OrderId, result.Symbol, result.Status)
		}
	}
	return err
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) (orders []types.Order, err error) {
	v := url.Values{}
	v.Set("symbol", symbol)
	var results []cancelOrderResp
	if err = e.signRequest(ctx, "DELETE", "/openOrders", v, &results); err != nil {
		log.WithError(err).Errorf("cancel by symbol failed")
		return orders, err
	}
	for _, result := range results {
		if result.Status != types.OrderStatusCanceled &&
			result.Status != types.OrderStatusPartiallyCanceled {
			log.Debugf("cancel failed: OrderId: %s, Symbol: %s, Status: %s",
				result.OrderId, result.Symbol, result.Status)
		} else {
			log.Debugf("cancel succeed: OrderId: %s, Symbol: %s, Status: %s",
				result.OrderId, result.Symbol, result.Status)
		}
		orders = append(orders, result.ToOrder())
	}
	return orders, err
}

type queryOrderResp struct {
	cancelOrderResp
	StopPrice  fixedpoint.Value `json:"stopPrice"`
	Time       int64            `json:"time"`
	UpdateTime int64            `json:"updateTime"`
	IsWorking  bool             `json:"isWorking"`

	// open order query
	IcebergQty        fixedpoint.Value `json:"icebergQty"`
	OrigQuoteOrderQty fixedpoint.Value `json:origQuoteOrderQty`
	OrderListId       int64            `json:"orderListId"`
}

func (q *queryOrderResp) ToOrder() types.Order {
	order := q.cancelOrderResp.ToOrder()
	order.StopPrice = q.StopPrice
	order.CreationTime = types.Time(types.NewMillisecondTimestampFromInt(q.Time))
	order.UpdateTime = types.Time(types.NewMillisecondTimestampFromInt(q.UpdateTime))
	order.IsWorking = q.IsWorking
	return order
}

/*{"symbol":"USTUSDT","orderId":"156198093501521920","orderListId":-1,"clientOrderId":null,"price":"0.01","origQty":"2799.81","executedQty":"0","cummulativeQuoteQty":"0","status":"NEW","timeInForce":null,"type":"LIMIT","side":"BUY","stopPrice":null,"icebergQty":null,"time":1653022763000,"updateTime":1653022763000,"isWorking":true,"origQuoteOrderQty":"27.9981"}*/

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	v := url.Values{}
	v.Set("symbol", q.Symbol)
	if len(q.OrderID) > 0 {
		v.Set("orderId", q.OrderID)
	}
	if len(q.ClientOrderID) > 0 {
		v.Set("origClientId", q.ClientOrderID)
	}

	var result queryOrderResp
	if err := e.signRequest(ctx, "GET", "/order", v, &result); err != nil {
		log.WithError(err).Errorf("query order failed")
		return nil, err
	}
	order := result.ToOrder()
	return &order, nil
}

/*[{"symbol":"USTUSDT","orderId":"156198093501521920","orderListId":-1,"clientOrderId":"","price":"0.01","origQty":"2799.81","executedQty":"0","cummulativeQuoteQty":"0","status":"NEW","timeInForce":null,"type":"LIMIT","side":"BUY","stopPrice":null,"icebergQty":null,"time":1653022763377,"updateTime":null,"isWorking":true,"origQuoteOrderQty":"27.9981"}]*/

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	v := url.Values{}
	v.Set("symbol", symbol)
	var results []queryOrderResp
	if err = e.signRequest(ctx, "GET", "/openOrders", v, &results); err != nil {
		log.WithError(err).Errorf("open orders query failed")
		return orders, err
	}
	for _, result := range results {
		orders = append(orders, result.ToOrder())
	}
	return orders, err
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	v := url.Values{}
	v.Set("symbol", symbol)
	v.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	v.Set("endTime", strconv.FormatInt(until.UnixMilli(), 10))
	if lastOrderID > 0 {
		v.Set("orderId", strconv.FormatUint(lastOrderID, 10))
	}
	var results []queryOrderResp
	if err = e.signRequest(ctx, "GET", "/allOrders", v, &results); err != nil {
		return orders, err
	}
	for _, result := range results {
		if result.Status != types.OrderStatusNew { // filled or cancelled
			orders = append(orders, result.ToOrder())
		}
	}
	return orders, err
}

// TODO: test endpoint integration

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	for _, o := range orders {
		v := url.Values{}
		v.Set("symbol", o.Symbol)
		// BUY, SELL
		v.Set("side", string(o.Side))
		// LIMIT, MARKET, LIMIT_MAKER
		v.Set("type", string(o.Type))
		if o.Type == types.OrderTypeLimit || o.Type == types.OrderTypeLimitMaker {
			if o.Quantity.IsZero() {
				return createdOrders, errors.New("order quantity error: limit order should have quantity set")
			}
			if o.Market.Symbol != "" {
				v.Set("quantity", o.Market.FormatQuantity(o.Quantity))
			} else {
				v.Set("quantity", o.Quantity.FormatString(8))
			}
		} else { // Market
			if o.Side == types.SideTypeBuy {
				if o.QuoteOrderQty.IsZero() {
					return createdOrders, errors.New("order quoteqty error: market buy order should have quoteOrderQty set")
				}
				if o.Market.Symbol != "" {
					v.Set("quoteOrderQty", o.Market.FormatVolume(o.QuoteOrderQty))
				} else {
					v.Set("quoteOrderQty", o.QuoteOrderQty.FormatString(8))
				}
			} else {
				if o.Quantity.IsZero() {
					return createdOrders, errors.New("order quantity error: market sell order should have quantity set")
				}
				if o.Market.Symbol != "" {
					v.Set("quantity", o.Market.FormatQuantity(o.Quantity))
				} else {
					v.Set("quantity", o.Quantity.FormatString(8))
				}
			}
		}
		switch o.Type {
		case types.OrderTypeLimit, types.OrderTypeLimitMaker:
			if o.Market.Symbol != "" {
				v.Set("price", o.Market.FormatPrice(o.Price))
			} else {
				v.Set("price", o.Price.FormatString(8))
			}
		case types.OrderTypeStopLimit:
			return createdOrders, errors.New("stop limit order not supported")
		}
		// TODO: broker ID for newClientOrderId
		var result queryOrderResp
		if err = e.signRequest(ctx, "POST", "/order", v, &result); err != nil {
			log.WithError(err).Errorf("submit orders failed")
			return createdOrders, err
		}
		order := result.ToOrder()
		order.SubmitOrder = o
		createdOrders = append(createdOrders, order)
	}
	return createdOrders, err
}

type queryTradeResp struct {
	Symbol      string           `json:"symbol"`
	Id          uint64           `json:"id"`
	OrderId     uint64           `json:"orderId"`
	OrderListId int64            `json:"orderListId"`
	Price       fixedpoint.Value `json:"price"`
	Qty         fixedpoint.Value `json:"qty"`
	QuoteQty    fixedpoint.Value `json:"quoteQty"`
	Time        int64            `json:"time"`
	//commission
	//commissionAsset
	IsBuyer     bool `json:"isBuyer"`
	IsMaker     bool `json:"isMaker"`
	IsBestMatch bool `json:"isBestMatch"`
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	v := url.Values{}
	v.Set("symbol", symbol)
	if options != nil {
		if options.StartTime != nil {
			v.Set("startTime", strconv.FormatInt(options.StartTime.UnixMilli(), 10))
		}
		if options.EndTime != nil {
			v.Set("endTime", strconv.FormatInt(options.EndTime.UnixMilli(), 10))
		}
		if options.Limit != 0 {
			v.Set("limit", strconv.FormatInt(options.Limit, 10))
		}
		// TODO: Not documented, need check
		// there's only orderId search option
		if options.LastTradeID != 0 {
			v.Set("id", strconv.FormatUint(options.LastTradeID, 10))
		}
	}
	var results []queryTradeResp
	if err = e.signRequest(ctx, "GET", "/myTrades", v, &results); err != nil {
		log.WithError(err).Errorf("submit orders failed")
		return trades, err
	}
	return trades, err

}

type Balance struct {
	Asset  string           `json:"asset"`
	Free   fixedpoint.Value `json:"free"`
	Locked fixedpoint.Value `json:"locked"`
}

func (b *Balance) ToBalance() types.Balance {
	return types.Balance{
		Currency:  b.Asset,
		Available: b.Free,
		Locked:    b.Locked,
	}
}

type queryAccountResp struct {
	MakerCommission  fixedpoint.Value `json:"makerCommission"`
	TakerCommission  fixedpoint.Value `json:"takerCommission"`
	SellerCommission fixedpoint.Value `json:"sellerCommission"`
	CanTrade         bool             `json:"canTrade"`
	CanWithdraw      bool             `json:"canWithdraw"`
	CanDeposit       bool             `josn:"candeposit"`
	UpdateTime       *time.Time       `json:"updateTime"`
	AccountType      string           `json:"accountType"`
	Balances         []Balance        `json:"balances"`
	Permissions      []string         `json:"permissions"`
}

/*{"makerCommission":20,"takerCommission":20,"buyerCommission":0,"sellerCommission":0,"canTrade":true,"canWithdraw":true,"canDeposit":true,"updateTime":null,"accountType":"SPOT","balances":[{"asset":"USDT","free":"22.6476628710420514","locked":"0"},{"asset":"AZERO3S","free":"0.0087","locked":"0"},{"asset":"AZERO","free":"0.00464","locked":"0"}],"permissions":["SPOT"]}*/

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	var result queryAccountResp
	if err := e.signRequest(ctx, "GET", "/account", url.Values{}, &result); err != nil {
		log.WithError(err).Errorf("query account fail")
		return nil, err
	}
	balances := make(map[string]types.Balance)
	for _, b := range result.Balances {
		balances[b.Asset] = b.ToBalance()
	}
	a := types.NewAccount()
	a.UpdateBalances(balances)
	a.AccountType = types.AccountType(strings.ToLower(result.AccountType))
	a.MakerFeeRate = result.MakerCommission
	a.TakerFeeRate = result.TakerCommission
	a.CanTrade = result.CanTrade
	a.CanWithdraw = result.CanWithdraw
	a.CanDeposit = result.CanDeposit

	return a, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	var result queryAccountResp
	if err := e.signRequest(ctx, "GET", "/account", url.Values{}, &result); err != nil {
		log.WithError(err).Errorf("query account fail")
		return nil, err
	}
	balances := make(map[string]types.Balance)
	for _, b := range result.Balances {
		balances[b.Asset] = b.ToBalance()
	}
	return balances, nil
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e)
}

/*
func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
}

func (e *Exchange) Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
}

func (e *Exchange) QueryRewards(ctx context.Context, startTime time.Time) ([]types.Reward, error) {
}
*/
var _ types.Exchange = &Exchange{}
