package btc

const (
	// HalvingInterval is the number of blocks between subsidy halvings.
	HalvingInterval = 210_000

	// CoinbaseMaturity is how many confirmations a coinbase output needs
	// before it can be spent: a reorg would make it disappear.
	CoinbaseMaturity = 100

	initialSubsidy Amount = 50 * SatoshiPerBitcoin
)

// BlockSubsidy returns the newly created coins a block at height may pay to
// its miner, on top of the fees of its transactions: 50 BTC halved every
// HalvingInterval blocks, and nothing once it has been halved 64 times.
func BlockSubsidy(height int64) Amount {
	halvings := height / HalvingInterval
	if height < 0 || halvings >= 64 {
		return 0
	}

	return initialSubsidy >> halvings
}
