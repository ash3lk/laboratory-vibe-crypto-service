package collector

// CoinMeta holds all source-specific identifiers for one coin.
type CoinMeta struct {
	Symbol      string // canonical ticker, e.g. "BTC"
	Name        string // human name, e.g. "Bitcoin"
	BinancePair string // Binance USDT pair, e.g. "BTCUSDT"
	GeckoID     string // CoinGecko coin id, e.g. "bitcoin"
}

// Registry is the master list of every coin this service knows about.
// To add a new coin: append one entry here — no other code changes needed.
var Registry = []CoinMeta{
	{Symbol: "BTC",  Name: "Bitcoin",       BinancePair: "BTCUSDT",   GeckoID: "bitcoin"},
	{Symbol: "ETH",  Name: "Ethereum",      BinancePair: "ETHUSDT",   GeckoID: "ethereum"},
	{Symbol: "SOL",  Name: "Solana",        BinancePair: "SOLUSDT",   GeckoID: "solana"},
	{Symbol: "BNB",  Name: "BNB",           BinancePair: "BNBUSDT",   GeckoID: "binancecoin"},
	{Symbol: "XRP",  Name: "XRP",           BinancePair: "XRPUSDT",   GeckoID: "ripple"},
	{Symbol: "ADA",  Name: "Cardano",       BinancePair: "ADAUSDT",   GeckoID: "cardano"},
	{Symbol: "DOGE", Name: "Dogecoin",      BinancePair: "DOGEUSDT",  GeckoID: "dogecoin"},
	{Symbol: "AVAX", Name: "Avalanche",     BinancePair: "AVAXUSDT",  GeckoID: "avalanche-2"},
	{Symbol: "DOT",  Name: "Polkadot",      BinancePair: "DOTUSDT",   GeckoID: "polkadot"},
	{Symbol: "LINK", Name: "Chainlink",     BinancePair: "LINKUSDT",  GeckoID: "chainlink"},
	{Symbol: "MATIC",Name: "Polygon",       BinancePair: "MATICUSDT", GeckoID: "matic-network"},
	{Symbol: "UNI",  Name: "Uniswap",       BinancePair: "UNIUSDT",   GeckoID: "uniswap"},
	{Symbol: "ATOM", Name: "Cosmos",        BinancePair: "ATOMUSDT",  GeckoID: "cosmos"},
	{Symbol: "LTC",  Name: "Litecoin",      BinancePair: "LTCUSDT",   GeckoID: "litecoin"},
	{Symbol: "TRX",  Name: "TRON",          BinancePair: "TRXUSDT",   GeckoID: "tron"},
}

// registryBySymbol is a fast lookup map built at init time.
var registryBySymbol map[string]CoinMeta

func init() {
	registryBySymbol = make(map[string]CoinMeta, len(Registry))
	for _, c := range Registry {
		registryBySymbol[c.Symbol] = c
	}
}

// Lookup returns CoinMeta for a symbol (case-sensitive). ok=false if unknown.
func Lookup(symbol string) (CoinMeta, bool) {
	c, ok := registryBySymbol[symbol]
	return c, ok
}
