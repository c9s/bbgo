package curve

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// ErrReadOnly is returned by Swap when the client has no sender.
var ErrReadOnly = errors.New("this curve client is read-only, configure a private key on the chain session")

// ErrNoPool is returned when no pool trades a pair.
var ErrNoPool = errors.New("no curve pool trades this pair")

// ErrNoQuote is returned when pools exist but none of them quotes the size.
var ErrNoQuote = errors.New("no curve pool quotes this size")

// DefaultMaxQuotedPools bounds how many pools a best-quote search calls. Curve
// lists 125 pools for USDC/USDT on Ethereum alone, most of them dust, so the
// search quotes the deepest ones and stops as soon as no remaining pool can
// beat the best quote found.
const DefaultMaxQuotedPools = 16

// TxSender is the transaction-sending capability the swap needs. *evm.Sender
// implements it.
type TxSender = token.TxSender

// Options configures a Client.
type Options struct {
	// AddressProvider overrides Curve's AddressProvider, for a chain where it
	// is deployed elsewhere.
	AddressProvider common.Address

	// MetaRegistry skips the AddressProvider lookup entirely.
	MetaRegistry common.Address

	// Sender enables Swap.
	Sender TxSender

	// Tokens resolves token metadata; without it, metadata is read directly.
	Tokens *token.Registry

	// Allowance is the approval policy used before swapping.
	Allowance token.AllowanceConfig

	// Multicall overrides the chain's Multicall3 address.
	Multicall common.Address

	// MaxQuotedPools bounds the best-quote search; zero means
	// DefaultMaxQuotedPools.
	MaxQuotedPools int
}

// Client is Curve on one chain, reached through that chain's MetaRegistry.
type Client struct {
	chain     *evm.Chain
	caller    bind.ContractCaller
	sender    TxSender
	tokens    *token.Registry
	allowance token.AllowanceConfig
	multicall common.Address

	metaRegistry common.Address
	poolCount    int64
	maxQuoted    int

	mu sync.Mutex

	// indexKinds caches the detected coin index type per pool, which costs one
	// extra call the first time a pool is quoted and nothing afterwards.
	indexKinds map[common.Address]IndexKind

	// scanned is the fallback coin index, built only on a chain whose
	// MetaRegistry cannot answer find_pools_for_coins.
	scanned []scannedPool
}

// scannedPool is one pool and its coins, as collected by the fallback scan.
type scannedPool struct {
	address    common.Address
	coins      []common.Address
	underlying []common.Address
}

// NewClient resolves the MetaRegistry from the AddressProvider and verifies
// that it answers, so a chain without Curve fails here rather than as an empty
// pool list later.
func NewClient(
	ctx context.Context, chain *evm.Chain, caller bind.ContractCaller, options Options,
) (*Client, error) {
	multicallAddress := options.Multicall
	if multicallAddress == (common.Address{}) {
		multicallAddress = chain.Multicall3
	}

	maxQuoted := options.MaxQuotedPools
	if maxQuoted <= 0 {
		maxQuoted = DefaultMaxQuotedPools
	}

	client := &Client{
		chain:        chain,
		caller:       caller,
		sender:       nonNilSender(options.Sender),
		tokens:       options.Tokens,
		allowance:    options.Allowance,
		multicall:    multicallAddress,
		metaRegistry: options.MetaRegistry,
		maxQuoted:    maxQuoted,
		indexKinds:   map[common.Address]IndexKind{},
	}

	if client.metaRegistry == (common.Address{}) {
		provider := options.AddressProvider
		if provider == (common.Address{}) {
			provider = AddressProvider()
		}

		if err := requireCode(ctx, caller, provider, "curve address provider", chain); err != nil {
			return nil, err
		}

		values, err := call(ctx, caller, addressProviderABI, provider,
			"get_address", big.NewInt(MetaRegistryID))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve the curve metaregistry from %s: %w",
				provider.Hex(), err)
		}

		registry, err := asAddress(values, 0)
		if err != nil {
			return nil, err
		}

		if registry == (common.Address{}) {
			return nil, fmt.Errorf(
				"the curve address provider at %s on %s has no metaregistry at id %d",
				provider.Hex(), chain.Name, MetaRegistryID)
		}

		client.metaRegistry = registry
	}

	if err := requireCode(ctx, caller, client.metaRegistry, "curve metaregistry", chain); err != nil {
		return nil, err
	}

	values, err := call(ctx, caller, metaRegistryABI, client.metaRegistry, "pool_count")
	if err != nil {
		return nil, fmt.Errorf("the curve metaregistry at %s does not answer pool_count: %w",
			client.metaRegistry.Hex(), err)
	}

	count, err := asBig(values, 0)
	if err != nil {
		return nil, err
	}

	client.poolCount = count.Int64()

	log.Infof("curve: metaregistry %s on %s indexes %d pools",
		client.metaRegistry.Hex(), chain.Name, client.poolCount)

	return client, nil
}

func requireCode(
	ctx context.Context, caller bind.ContractCaller, address common.Address, name string, chain *evm.Chain,
) error {
	code, err := caller.CodeAt(ctx, address, nil)
	if err != nil {
		return fmt.Errorf("failed to check the %s at %s: %w", name, address.Hex(), err)
	}

	if len(code) == 0 {
		return fmt.Errorf("no contract is deployed at the %s address %s on %s",
			name, address.Hex(), chain.Name)
	}

	return nil
}

// nonNilSender guards against a typed nil, which a caller produces by passing a
// nil *evm.Sender into the interface field.
func nonNilSender(sender TxSender) TxSender {
	if sender == nil {
		return nil
	}

	value := reflect.ValueOf(sender)
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		if value.IsNil() {
			return nil
		}
	}

	return sender
}

// MetaRegistryAddress is the registry every read goes through.
func (c *Client) MetaRegistryAddress() common.Address {
	return c.metaRegistry
}

// PoolCount is the number of pools the registry indexed at construction.
func (c *Client) PoolCount() int64 {
	return c.poolCount
}

// ChainName is the name of the chain this client talks to.
func (c *Client) ChainName() string {
	return c.chain.Name
}

// CanWrite reports whether this client can send transactions.
func (c *Client) CanWrite() bool {
	return c.sender != nil
}

func (c *Client) requireSender() (TxSender, error) {
	if c.sender == nil {
		return nil, ErrReadOnly
	}

	return c.sender, nil
}

// Token resolves the metadata of a token, through the registry when there is
// one and directly from the chain otherwise.
func (c *Client) Token(ctx context.Context, address common.Address) (token.Token, error) {
	if c.tokens != nil {
		return c.tokens.Lookup(ctx, address)
	}

	decimals, err := token.Decimals(ctx, c.caller, address)
	if err != nil {
		return token.Token{}, err
	}

	t := token.Token{Address: address, Decimals: decimals, Symbol: address.Hex()}
	if symbol, err := token.Symbol(ctx, c.caller, address); err == nil && symbol != "" {
		t.Symbol = symbol
	}

	return t, nil
}

// Routes resolves every way to trade one ordered pair, deepest output balance
// first.
//
// For a direct route the ordering is not a heuristic: a pool cannot pay out
// more of a coin than it holds, so its balance of the output coin is an upper
// bound on the swap, and a pool holding less than the best quote so far cannot
// win. Quotes stops on exactly that condition. An underlying route carries a
// balance the registry does not convert, so it is ordered but not bounded; see
// Route.OutBalanceExact.
func (c *Client) Routes(ctx context.Context, from, to common.Address) ([]*Route, error) {
	addresses, err := c.FindPoolAddresses(ctx, from, to)
	if err != nil {
		return nil, err
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("%w: %s / %s on %s", ErrNoPool, from.Hex(), to.Hex(), c.chain.Name)
	}

	const callsPerRoute = 3

	calls := make([]multicall.Call, 0, len(addresses)*callsPerRoute)
	for _, address := range addresses {
		for _, method := range []string{methodCoinIndices, methodBalances, methodUnderlyingBalances} {
			var (
				data []byte
				err  error
			)

			if method == methodCoinIndices {
				data, err = metaRegistryABI.Pack(method, address, from, to)
			} else {
				data, err = metaRegistryABI.Pack(method, address)
			}

			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{
				Target: c.metaRegistry, AllowFailure: true, CallData: data,
			})
		}
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls,
		callsPerRoute*poolsPerRouteBatch)
	if err != nil {
		return nil, err
	}

	routes := make([]*Route, 0, len(addresses))

	for i, address := range addresses {
		window := results[i*callsPerRoute : (i+1)*callsPerRoute]

		route, err := c.decodeRoute(address, from, to, window)
		if err != nil {
			log.Debugf("curve: skipping the pool at %s: %s", address.Hex(), err)
			continue
		}

		routes = append(routes, route)
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("%w: %s / %s on %s, no indexed pool reports the pair",
			ErrNoPool, from.Hex(), to.Hex(), c.chain.Name)
	}

	sort.SliceStable(routes, func(a, b int) bool {
		return routes[a].OutBalance.Cmp(routes[b].OutBalance) > 0
	})

	return routes, nil
}

func (c *Client) decodeRoute(
	pool, from, to common.Address, results []multicall.Result,
) (*Route, error) {
	if !results[0].Success || len(results[0].ReturnData) == 0 {
		return nil, fmt.Errorf("get_coin_indices reverted")
	}

	values, err := metaRegistryABI.Unpack(methodCoinIndices, results[0].ReturnData)
	if err != nil {
		return nil, err
	}

	i, err := asBig(values, 0)
	if err != nil {
		return nil, err
	}

	j, err := asBig(values, 1)
	if err != nil {
		return nil, err
	}

	underlying, err := asBool(values, 2)
	if err != nil {
		return nil, err
	}

	if i.Cmp(j) == 0 {
		return nil, fmt.Errorf("the registry reports the same index for both coins")
	}

	// a metapool's underlying trade is bounded by the base pool's balances,
	// which get_underlying_balances reports and get_balances does not
	balanceMethod, balanceResult := methodBalances, results[1]
	if underlying {
		balanceMethod, balanceResult = methodUnderlyingBalances, results[2]
	}

	outBalance := new(big.Int)

	if balanceResult.Success && len(balanceResult.ReturnData) > 0 {
		balanceValues, err := metaRegistryABI.Unpack(balanceMethod, balanceResult.ReturnData)
		if err == nil && len(balanceValues) > 0 {
			balances := decodeUintArray(balanceValues[0], MaxCoins)
			if index := int(j.Int64()); index >= 0 && index < len(balances) {
				outBalance = balances[index]
			}
		}
	}

	route := &Route{
		Pool:            pool,
		TokenIn:         from,
		TokenOut:        to,
		I:               i.Int64(),
		J:               j.Int64(),
		Underlying:      underlying,
		Index:           c.cachedIndexKind(pool),
		OutBalance:      outBalance,
		OutBalanceExact: !underlying,
	}

	return route, nil
}

func (c *Client) cachedIndexKind(pool common.Address) IndexKind {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.indexKinds[pool]
}

func (c *Client) rememberIndexKind(pool common.Address, kind IndexKind) {
	if kind == IndexUnknown {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.indexKinds[pool] = kind
}

// indexCandidates is the order in which an undetected pool is probed.
func indexCandidates(kind IndexKind) []IndexKind {
	if kind != IndexUnknown {
		return []IndexKind{kind}
	}

	return []IndexKind{IndexInt128, IndexUint256}
}

// Quote asks one pool what an exact-input swap would return.
//
// When the pool's index type is not known yet, both are tried in one batched
// call and the one that answers is remembered.
func (c *Client) Quote(ctx context.Context, route *Route, amountIn *big.Int) (*Quote, error) {
	if route == nil {
		return nil, fmt.Errorf("a route is required")
	}

	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("the input amount must be positive")
	}

	quotes, err := c.quoteRoutes(ctx, []*Route{route}, amountIn)
	if err != nil {
		return nil, err
	}

	if len(quotes) == 0 || quotes[0] == nil {
		return nil, fmt.Errorf("%w: the pool at %s did not answer %s",
			ErrNoQuote, route.Pool.Hex(), route.QuoteMethod())
	}

	return quotes[0], nil
}

// quoteRoutes quotes a group of routes in one batched call. The returned slice
// matches the input order and holds a nil for a route that did not answer.
func (c *Client) quoteRoutes(
	ctx context.Context, routes []*Route, amountIn *big.Int,
) ([]*Quote, error) {
	if len(routes) == 0 {
		return nil, nil
	}

	type plan struct {
		route  *Route
		kinds  []IndexKind
		offset int
	}

	plans := make([]plan, 0, len(routes))
	calls := make([]multicall.Call, 0, len(routes)*2)

	for _, route := range routes {
		kinds := indexCandidates(route.Index)

		p := plan{route: route, kinds: kinds, offset: len(calls)}

		for _, kind := range kinds {
			data, err := PoolABI(kind).Pack(route.QuoteMethod(),
				big.NewInt(route.I), big.NewInt(route.J), amountIn)
			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{
				Target: route.Pool, AllowFailure: true, CallData: data,
			})
		}

		plans = append(plans, p)
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	quotes := make([]*Quote, len(routes))

	for index, p := range plans {
		for k, kind := range p.kinds {
			amountOut, ok := decodeQuoteResult(results[p.offset+k], kind, p.route.QuoteMethod())
			if !ok {
				continue
			}

			p.route.Index = kind
			c.rememberIndexKind(p.route.Pool, kind)

			quotes[index] = &Quote{
				Route:     p.route,
				AmountIn:  new(big.Int).Set(amountIn),
				AmountOut: amountOut,
			}

			break
		}
	}

	return quotes, nil
}

// decodeQuoteResult reads one get_dy result, and reports whether the pool
// actually answered.
//
// A pool that does not have the selector usually reverts, which arrives as an
// unsuccessful result, but not always: a tricrypto-ng pool asked with int128
// indices answers *successfully* with an empty payload. So success alone does
// not mean the index type was right, and a usable answer is one that is
// successful, long enough to hold a word, decodes, and is positive.
func decodeQuoteResult(result multicall.Result, kind IndexKind, method string) (*big.Int, bool) {
	if !result.Success || len(result.ReturnData) < 32 {
		return nil, false
	}

	values, err := PoolABI(kind).Unpack(method, result.ReturnData)
	if err != nil || len(values) == 0 {
		return nil, false
	}

	amountOut, err := asBig(values, 0)
	if err != nil || amountOut.Sign() <= 0 {
		return nil, false
	}

	return amountOut, true
}

// Quotes quotes the pools that trade a pair for one size, best output first.
//
// It walks the routes in descending output balance and quotes them in batches,
// stopping once the next pool's balance of the output coin cannot beat the best
// quote already found. MaxQuotedPools caps the work on a pair with a long tail
// of dust pools.
//
// Every quote is the pool's own arithmetic, which is not a promise that the
// swap can be made: a deprecated pool of wrapped coins that can no longer be
// redeemed keeps quoting a price it cannot honour, and the Curve front end
// hides such pools behind a whitelist this adapter deliberately does not carry.
// What protects a swap is the transaction layer's mandatory simulation, which
// reverts before broadcasting, and min_dy, which makes a fill worse than the
// floor impossible. Showing the whole table rather than only the winner is what
// lets an operator see the difference.
func (c *Client) Quotes(
	ctx context.Context, from, to common.Address, amountIn *big.Int,
) ([]*Quote, error) {
	routes, err := c.Routes(ctx, from, to)
	if err != nil {
		return nil, err
	}

	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("the input amount must be positive")
	}

	const groupSize = 8

	// the bound can only be used once no route left to visit carries an
	// inexact one
	lastInexact := -1

	for i, route := range routes {
		if !route.OutBalanceExact {
			lastInexact = i
		}
	}

	var (
		best    *Quote
		out     []*Quote
		quoted  int
		skipped int
	)

	for start := 0; start < len(routes) && quoted < c.maxQuoted; start += groupSize {
		end := start + groupSize
		if end > len(routes) {
			end = len(routes)
		}

		group := routes[start:end]

		// every remaining route holds less of the output coin than the best
		// quote, so none of them can win
		if best != nil && start > lastInexact && group[0].OutBalance.Cmp(best.AmountOut) <= 0 {
			skipped = len(routes) - start
			break
		}

		if quoted+len(group) > c.maxQuoted {
			group = group[:c.maxQuoted-quoted]
		}

		quotes, err := c.quoteRoutes(ctx, group, amountIn)
		if err != nil {
			return nil, err
		}

		quoted += len(group)

		for _, quote := range quotes {
			if quote == nil {
				continue
			}

			out = append(out, quote)

			if best == nil || quote.AmountOut.Cmp(best.AmountOut) > 0 {
				best = quote
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("%w: %s / %s on %s, %d pools asked",
			ErrNoQuote, from.Hex(), to.Hex(), c.chain.Name, quoted)
	}

	sort.SliceStable(out, func(a, b int) bool {
		return out[a].AmountOut.Cmp(out[b].AmountOut) > 0
	})

	log.Debugf("curve: best of %d pools quoted (%d skipped as too shallow): %s",
		quoted, skipped, best)

	return out, nil
}

// BestQuote returns the pool that quotes the most for one size. See Quotes for
// what a quote does and does not guarantee.
func (c *Client) BestQuote(
	ctx context.Context, from, to common.Address, amountIn *big.Int,
) (*Quote, error) {
	quotes, err := c.Quotes(ctx, from, to, amountIn)
	if err != nil {
		return nil, err
	}

	return quotes[0], nil
}

// RouteForPool resolves one named pool into a route for a pair, which is what
// pinning a swap to a pool needs.
func (c *Client) RouteForPool(
	ctx context.Context, pool, from, to common.Address,
) (*Route, error) {
	values, err := call(ctx, c.caller, metaRegistryABI, c.metaRegistry,
		methodCoinIndices, pool, from, to)
	if err != nil {
		return nil, fmt.Errorf("the pool at %s does not trade this pair: %w", pool.Hex(), err)
	}

	i, err := asBig(values, 0)
	if err != nil {
		return nil, err
	}

	j, err := asBig(values, 1)
	if err != nil {
		return nil, err
	}

	underlying, err := asBool(values, 2)
	if err != nil {
		return nil, err
	}

	if i.Cmp(j) == 0 {
		return nil, fmt.Errorf("the pool at %s reports the same index for both coins", pool.Hex())
	}

	return &Route{
		Pool:       pool,
		TokenIn:    from,
		TokenOut:   to,
		I:          i.Int64(),
		J:          j.Int64(),
		Underlying: underlying,
		Index:      c.cachedIndexKind(pool),
		OutBalance: new(big.Int),
		// a pinned pool is not ordered against anything, so no balance is read
		OutBalanceExact: false,
	}, nil
}

// A batched read is bounded by the bytes it brings back, not only by the number
// of calls: a public RPC node caps the data one eth_call may return, and
// Polygon's Bor nodes cap it at 100,000 bytes. One pool's metadata window is
// about 1.5 KB, so a pair with 100 pools is well past that on the default batch
// size. These are the batch sizes in pools, converted to calls at each use.
const (
	poolsPerMetadataBatch = 24
	poolsPerRouteBatch    = 48
	poolsPerScanBatch     = 64
)

// The registry methods that more than one read uses.
const (
	methodCoinIndices        = "get_coin_indices"
	methodBalances           = "get_balances"
	methodUnderlyingBalances = "get_underlying_balances"
)
