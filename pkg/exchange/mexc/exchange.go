package mexc

import (

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"time"
	"crypto/tls"
	"crypto/hmac"
	"net/http"
	"net/url"
)

const MX = "MX"
const apiURL = "https://api.mexc.com/api/v3"
const urlTemplate = net.URL{
	Scheme: "https",
	Host: apiURL,
}

var log = logrus.WithFields("exchange", "mexc")

type Exchange struct {
	key, secret string
	client *http.Client
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeMEXC
}

func (e *Exchange) PlatformFeeCurrency() string {
	return MX
}

func (e *Exchange) publicRequest(ctx context.Context, method string, path string, params url.Values) (*http.Response, error) {
	u := urlTemplate
	u.Path = path
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String())
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client {
			Transport: &http.Transport{
				MaxIdleConns: 10,
				IdleConnTimeout: 30 * time.Second,
			}
		}
	}
	return e.client.Do(req)
}

func (e *Exchange) signRequest(ctx context.Context, method string, path string, params url.Values) (*http.Response, error) {
	timestamp := strconv.Itoa(time.Now().Unix())
	params.Add("timestamp", timestamp)
	queryString := params.Encode()
	sign := hmac.New(sha256.New, []byte(e.secret))
	sign.Write(queryString)
	signature := fmt.Sprintf("%x", sign.Sum64())
	params.Add("signature", signature)
	u := urlTemplate
	u.Path = path
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String())
	if err != nil {
		return nil, err
	}
	req.Header.Add("Context-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client {
			Transport: &http.Transport {
				MaxIdleConns: 10,
				IdleConnTimeout: 30 * time.Second,
			}
		}
	}
	return e.client.Do(req)
}

func (e *Exchange) ping() {
}

func (e *Exchange) time() {

}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
}

func (e *Exchange) NewStream() types.Stream {
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, error) {
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, util time.Time, lastOrderID uint64) ([]types.Order, error) {
}

func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
}

func (e *Exchange) CancelOrders(ctx context.Context, ordrs ...types.Order) (err error) {
}

func (e *Exchange) Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
}

func (e *Exchange) QueryAccountBalance(ctx contexxt.Context) (types.BalanceMap, error) {
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
}

func (e *Exchange) QueryRewards(ctx context.Context, startTime time.Time) ([]types.Reward, error) {
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
}

var _ types.Exchange = &Exchange{}
