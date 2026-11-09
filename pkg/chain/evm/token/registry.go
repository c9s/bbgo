package token

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// Token is the resolved metadata of an ERC20 token on one chain.
type Token struct {
	Address  common.Address
	Symbol   string
	Name     string
	Decimals int
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%s, %d decimals)", t.Symbol, t.Address.Hex(), t.Decimals)
}

// ToFixedPoint converts a raw base-unit amount of this token.
func (t Token) ToFixedPoint(raw *big.Int) (fixedpoint.Value, error) {
	return ToFixedPoint(raw, t.Decimals)
}

// ToRaw converts a fixedpoint value into a raw base-unit amount of this token.
func (t Token) ToRaw(v fixedpoint.Value) (*big.Int, error) {
	return ToRaw(v, t.Decimals)
}

// FormatRaw renders a raw base-unit amount of this token as a decimal string.
func (t Token) FormatRaw(raw *big.Int) string {
	return FormatRaw(raw, t.Decimals)
}

// Balance pairs a token with a raw base-unit balance.
type Balance struct {
	Token Token
	Raw   *big.Int
}

func (b Balance) String() string {
	return fmt.Sprintf("%s %s", b.Token.FormatRaw(b.Raw), b.Token.Symbol)
}

// Registry resolves and caches ERC20 metadata for one chain. Metadata is always
// read from the chain, never assumed: the same symbol has different decimals on
// different chains.
type Registry struct {
	caller    bind.ContractCaller
	multicall common.Address

	mu        sync.RWMutex
	byAddress map[common.Address]Token
	bySymbol  map[string]common.Address
}

func NewRegistry(caller bind.ContractCaller, multicallAddress common.Address) *Registry {
	return &Registry{
		caller:    caller,
		multicall: multicallAddress,
		byAddress: make(map[common.Address]Token),
		bySymbol:  make(map[string]common.Address),
	}
}

// AddSymbol binds a symbol to an address without resolving it. The metadata is
// fetched on first use.
func (r *Registry) AddSymbol(symbol string, address common.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySymbol[strings.ToUpper(symbol)] = address
}

// AddKnownSymbols registers the built-in symbol-to-address book of a chain.
func (r *Registry) AddKnownSymbols(chainID uint64) {
	for symbol, address := range Known(chainID) {
		r.AddSymbol(symbol, address)
	}
}

// Add stores an already resolved token, which also binds its symbol.
func (r *Registry) Add(t Token) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byAddress[t.Address] = t
	if t.Symbol != "" {
		r.bySymbol[strings.ToUpper(t.Symbol)] = t.Address
	}
}

// Cached returns the token metadata already resolved for an address.
func (r *Registry) Cached(address common.Address) (Token, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byAddress[address]
	return t, ok
}

// Symbols returns the registered symbols in sorted order.
func (r *Registry) Symbols() []string {
	r.mu.RLock()
	symbols := make([]string, 0, len(r.bySymbol))
	for symbol := range r.bySymbol {
		symbols = append(symbols, symbol)
	}
	r.mu.RUnlock()

	sort.Strings(symbols)
	return symbols
}

// Lookup resolves the metadata of a token address, reading symbol, name and
// decimals in a single batched call, and caches the result.
func (r *Registry) Lookup(ctx context.Context, address common.Address) (Token, error) {
	if t, ok := r.Cached(address); ok {
		return t, nil
	}

	calls := make([]multicall.Call, 0, 3)
	for _, method := range []string{"symbol", "name", "decimals"} {
		data, err := CallData(method)
		if err != nil {
			return Token{}, err
		}

		// symbol() and name() are optional in practice, decimals() is not
		calls = append(calls, multicall.Call{
			Target:       address,
			AllowFailure: method != "decimals",
			CallData:     data,
		})
	}

	results, err := multicall.Aggregate3(ctx, r.caller, r.multicall, calls)
	if err != nil {
		return Token{}, fmt.Errorf("failed to read erc20 metadata of %s: %w", address.Hex(), err)
	}

	token := Token{Address: address}

	if results[0].Success {
		if symbol, err := DecodeString("symbol", results[0].ReturnData); err == nil {
			token.Symbol = symbol
		}
	}

	if results[1].Success {
		if name, err := DecodeString("name", results[1].ReturnData); err == nil {
			token.Name = name
		}
	}

	decimals, err := DecodeDecimals(results[2].ReturnData)
	if err != nil {
		return Token{}, fmt.Errorf("failed to read decimals of %s: %w", address.Hex(), err)
	}

	token.Decimals = decimals

	if token.Symbol == "" {
		token.Symbol = address.Hex()
	}

	r.Add(token)

	return token, nil
}

// Resolve accepts either a hex address or a registered symbol.
func (r *Registry) Resolve(ctx context.Context, symbolOrAddress string) (Token, error) {
	s := strings.TrimSpace(symbolOrAddress)
	if s == "" {
		return Token{}, fmt.Errorf("empty token reference")
	}

	if common.IsHexAddress(s) {
		return r.Lookup(ctx, common.HexToAddress(s))
	}

	r.mu.RLock()
	address, ok := r.bySymbol[strings.ToUpper(s)]
	r.mu.RUnlock()

	if !ok {
		return Token{}, fmt.Errorf("unknown token %q, pass a contract address or register it in the chain session config", s)
	}

	return r.Lookup(ctx, address)
}

// ResolveAll resolves a list of symbols or addresses.
func (r *Registry) ResolveAll(ctx context.Context, refs []string) ([]Token, error) {
	tokens := make([]Token, 0, len(refs))
	for _, ref := range refs {
		t, err := r.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, t)
	}

	return tokens, nil
}

// BalanceOf reads one token balance.
func (r *Registry) BalanceOf(ctx context.Context, t Token, owner common.Address) (*big.Int, error) {
	return BalanceOf(ctx, r.caller, t.Address, owner)
}

// Balances reads several token balances in one batched call. Tokens whose
// balanceOf call fails are skipped, which keeps one broken token from failing
// the whole account read. Duplicate addresses are collapsed, because several
// symbols can point at the same contract, such as WPOL and WMATIC on Polygon.
func (r *Registry) Balances(ctx context.Context, owner common.Address, tokens []Token) ([]Balance, error) {
	tokens = dedupTokens(tokens)
	if len(tokens) == 0 {
		return nil, nil
	}

	calls := make([]multicall.Call, 0, len(tokens))
	for _, t := range tokens {
		data, err := CallData("balanceOf", owner)
		if err != nil {
			return nil, err
		}

		calls = append(calls, multicall.Call{Target: t.Address, AllowFailure: true, CallData: data})
	}

	results, err := multicall.Aggregate3Chunked(ctx, r.caller, r.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	balances := make([]Balance, 0, len(tokens))
	for i, result := range results {
		if !result.Success {
			continue
		}

		raw, err := DecodeUint256("balanceOf", result.ReturnData)
		if err != nil {
			continue
		}

		balances = append(balances, Balance{Token: tokens[i], Raw: raw})
	}

	return balances, nil
}

// dedupTokens collapses duplicate contract addresses, keeping the first
// occurrence so that the caller's ordering is preserved.
func dedupTokens(tokens []Token) []Token {
	seen := make(map[common.Address]struct{}, len(tokens))
	unique := make([]Token, 0, len(tokens))

	for _, t := range tokens {
		if _, ok := seen[t.Address]; ok {
			continue
		}

		seen[t.Address] = struct{}{}
		unique = append(unique, t)
	}

	return unique
}
