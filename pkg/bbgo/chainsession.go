package bbgo

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// ChainSession is the on-chain counterpart of ExchangeSession: one chain plus
// one address. To use several wallets on the same chain, define several
// sessions; a session never handles more than one address.
//
// The private key is never stored in the config file. Only the name of the
// environment variable holding it is, and it defaults to <SESSION>_PRIVATE_KEY.
// A session with no key is a read-only session, which is what the CLI,
// reconciliation and backtesting need.
type ChainSession struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Chain is the chain name or alias: ethereum, bsc or polygon.
	Chain string `json:"chain" yaml:"chain"`

	// RPCEndpoints is the ordered list of JSON-RPC endpoints. The first one is
	// preferred and the rest are used for failover. When empty, the endpoints
	// are read from <SESSION>_RPC_ENDPOINTS or <CHAIN>_RPC_ENDPOINTS.
	RPCEndpoints []string `json:"rpcEndpoints,omitempty" yaml:"rpcEndpoints,omitempty"`

	// WSEndpoint is reserved for the log subscription stream.
	WSEndpoint string `json:"wsEndpoint,omitempty" yaml:"wsEndpoint,omitempty"`

	// PrivateKeyVar overrides the name of the environment variable holding the
	// hex-encoded private key.
	PrivateKeyVar string `json:"privateKeyVar,omitempty" yaml:"privateKeyVar,omitempty"`

	// Address is the account address. With a private key configured it is
	// optional and, when given, it is verified against the key. Without a
	// private key it defines the read-only account to inspect.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// Confirmations overrides the per-chain default confirmation depth.
	Confirmations int `json:"confirmations,omitempty" yaml:"confirmations,omitempty"`

	// Tokens registers or overrides symbol to contract address bindings.
	// Decimals are always read from the chain, never from here.
	Tokens map[string]string `json:"tokens,omitempty" yaml:"tokens,omitempty"`

	chain  *evm.Chain
	client *evm.Client
	signer evm.Signer
	tokens *token.Registry

	address     common.Address
	initialized bool
}

func NewChainSession(chainName string, rpcEndpoints ...string) *ChainSession {
	return &ChainSession{
		Chain:        chainName,
		RPCEndpoints: rpcEndpoints,
	}
}

// Init resolves the chain, dials the RPC endpoints, loads the signer when a
// private key is available and prepares the token registry.
func (s *ChainSession) Init(ctx context.Context, name string) error {
	if s.initialized {
		return fmt.Errorf("chain session %s is already initialized", s.Name)
	}

	if s.Name == "" {
		s.Name = name
	}

	chain, ok := evm.LookupChain(s.Chain)
	if !ok {
		return fmt.Errorf("chain session %s: unknown chain %q, supported chains are %s",
			s.Name, s.Chain, strings.Join(evm.SupportedChains(), ", "))
	}

	s.chain = chain

	if s.Confirmations <= 0 {
		s.Confirmations = chain.DefaultConfirmations
	}

	endpoints := s.resolveEndpoints()
	if len(endpoints) == 0 {
		return fmt.Errorf("chain session %s: no rpc endpoint configured, set rpcEndpoints or the %s environment variable",
			s.Name, s.envVarName("RPC_ENDPOINTS"))
	}

	if err := s.loadSigner(); err != nil {
		return err
	}

	client, err := evm.NewClient(ctx, chain, endpoints)
	if err != nil {
		return fmt.Errorf("chain session %s: %w", s.Name, err)
	}

	s.client = client

	s.tokens = token.NewRegistry(client, chain.Multicall3)
	s.tokens.AddKnownSymbols(uint64(chain.ID))

	for symbol, address := range s.Tokens {
		if !common.IsHexAddress(address) {
			return fmt.Errorf("chain session %s: token %s has an invalid address %q", s.Name, symbol, address)
		}

		s.tokens.AddSymbol(symbol, common.HexToAddress(address))
	}

	s.initialized = true

	mode := "read-only"
	if s.signer != nil {
		mode = "signing"
	}

	log.Infof("chain session %s initialized: chain=%s address=%s mode=%s confirmations=%d endpoints=%v",
		s.Name, chain.Name, s.AddressHex(), mode, s.Confirmations, client.Endpoints())

	return nil
}

// resolveEndpoints falls back to the environment when the config has none.
func (s *ChainSession) resolveEndpoints() []string {
	if len(s.RPCEndpoints) > 0 {
		return s.RPCEndpoints
	}

	candidates := []string{
		s.envVarName("RPC_ENDPOINTS"),
		s.envVarName("RPC_URL"),
	}

	if s.chain != nil {
		candidates = append(candidates,
			envVarName(s.chain.Name, "RPC_ENDPOINTS"),
			envVarName(s.chain.Name, "RPC_URL"),
		)
	}

	for _, name := range candidates {
		if v, ok := os.LookupEnv(name); ok && strings.TrimSpace(v) != "" {
			return strings.Split(v, ",")
		}
	}

	return nil
}

func (s *ChainSession) loadSigner() error {
	keyVar := s.PrivateKeyVar
	if keyVar == "" {
		keyVar = s.envVarName("PRIVATE_KEY")
	}

	hexKey, found := os.LookupEnv(keyVar)
	if found && strings.TrimSpace(hexKey) != "" {
		signer, err := evm.NewPrivateKeySigner(hexKey)
		if err != nil {
			return fmt.Errorf("chain session %s: failed to load the private key from %s: %w", s.Name, keyVar, err)
		}

		s.signer = signer
		s.address = signer.Address()

		// a configured address that disagrees with the key is a configuration
		// error worth failing on: it means the wrong wallet would be used
		if s.Address != "" {
			if !common.IsHexAddress(s.Address) {
				return fmt.Errorf("chain session %s: invalid address %q", s.Name, s.Address)
			}

			if configured := common.HexToAddress(s.Address); configured != s.address {
				return fmt.Errorf("chain session %s: the configured address %s does not match the address %s derived from %s",
					s.Name, configured.Hex(), s.address.Hex(), keyVar)
			}
		}

		return nil
	}

	if s.Address != "" {
		if !common.IsHexAddress(s.Address) {
			return fmt.Errorf("chain session %s: invalid address %q", s.Name, s.Address)
		}

		s.address = common.HexToAddress(s.Address)
	}

	return nil
}

func (s *ChainSession) envVarName(suffix string) string {
	return envVarName(s.Name, suffix)
}

func envVarName(prefix, suffix string) string {
	prefix = strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(prefix)
	return strings.ToUpper(prefix) + "_" + suffix
}

func (s *ChainSession) Initialized() bool {
	return s.initialized
}

func (s *ChainSession) EVMChain() *evm.Chain {
	return s.chain
}

func (s *ChainSession) Client() *evm.Client {
	return s.client
}

// Signer returns the signer, or nil for a read-only session.
func (s *ChainSession) Signer() evm.Signer {
	return s.signer
}

func (s *ChainSession) CanSign() bool {
	return s.signer != nil
}

func (s *ChainSession) TokenRegistry() *token.Registry {
	return s.tokens
}

// EVMAddress returns the account address of this session. It is the zero
// address when neither a private key nor an address is configured.
func (s *ChainSession) EVMAddress() common.Address {
	return s.address
}

func (s *ChainSession) HasAddress() bool {
	return s.address != (common.Address{})
}

func (s *ChainSession) AddressHex() string {
	if !s.HasAddress() {
		return "(none)"
	}

	return s.address.Hex()
}

func (s *ChainSession) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

type ChainSessionMap map[string]*ChainSession

func (sessions ChainSessionMap) Names() []string {
	names := make([]string, 0, len(sessions))
	for name := range sessions {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func (sessions ChainSessionMap) Filter(names []string) ChainSessionMap {
	filtered := ChainSessionMap{}
	for _, name := range names {
		if session, ok := sessions[name]; ok {
			filtered[name] = session
		}
	}

	return filtered
}

func (sessions ChainSessionMap) Close() {
	for _, session := range sessions {
		session.Close()
	}
}
