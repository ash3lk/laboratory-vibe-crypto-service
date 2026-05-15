package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"example.com/crypto-collector/internal/model"
	"gorm.io/gorm"
)

// Collector fetches crypto prices on a schedule and persists them to the DB.
type Collector struct {
	db       *gorm.DB
	source   string
	interval time.Duration
	coins    []CoinMeta
	client   *http.Client
	stopCh   chan struct{}
	triggerCh chan struct{} // buffered channel for manual collection triggers
}

// New creates a Collector.
//   - source:      "binance" | "coingecko"
//   - intervalSec: collection cadence in seconds
//   - symbols:     validated against coins.go Registry; unknown ones are warned and dropped
func New(db *gorm.DB, source string, intervalSec int, symbols []string) *Collector {
	return &Collector{
		db:        db,
		source:    source,
		interval:  time.Duration(intervalSec) * time.Second,
		coins:     resolveCoins(symbols),
		client:    &http.Client{Timeout: 10 * time.Second},
		stopCh:    make(chan struct{}),
		triggerCh: make(chan struct{}, 1), // buffered: non-blocking sends
	}
}

// resolveCoins converts symbol strings → CoinMeta, dropping unknowns with a warning.
func resolveCoins(symbols []string) []CoinMeta {
	out := make([]CoinMeta, 0, len(symbols))
	for _, sym := range symbols {
		meta, ok := Lookup(sym)
		if !ok {
			log.Printf("[collector] WARNING: unknown coin %q — skipping (add it to coins.go)", sym)
			continue
		}
		out = append(out, meta)
	}
	if len(out) == 0 {
		log.Fatal("[collector] no valid coins configured — cannot start")
	}
	names := make([]string, len(out))
	for i, c := range out {
		names[i] = c.Symbol
	}
	log.Printf("[collector] tracking %d coins: %s", len(out), strings.Join(names, ", "))
	return out
}

// Start launches the background goroutine. Fires immediately, then on every tick.
func (c *Collector) Start(ctx context.Context) {
	log.Printf("[collector] starting — source=%s interval=%s coins=%d",
		c.source, c.interval, len(c.coins))

	go func() {
		c.collect()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.triggerCh:
				log.Println("[collector] manual trigger received")
				c.collect()
				// Reset ticker so the next automatic run is a full interval away
				ticker.Reset(c.interval)
			case <-c.stopCh:
				log.Println("[collector] stopped")
				return
			case <-ctx.Done():
				log.Println("[collector] context cancelled")
				return
			}
		}
	}()
}

// Stop signals the worker goroutine to exit.
func (c *Collector) Stop() { close(c.stopCh) }

// TriggerNow requests an immediate collection (non-blocking; ignored if one is already queued).
func (c *Collector) TriggerNow() {
	select {
	case c.triggerCh <- struct{}{}:
	default: // already queued, ignore
	}
}

// ---------------------------------------------------------------------------
// collect
// ---------------------------------------------------------------------------

func (c *Collector) collect() {
	log.Printf("[collector] collecting %d coins from %s", len(c.coins), c.source)

	var (
		records []model.Price
		err     error
	)

	switch c.source {
	case "binance":
		records, err = c.fetchBinance()
	case "coingecko":
		records, err = c.fetchCoinGecko()
	default:
		log.Printf("[collector] unknown source %q, skipping", c.source)
		return
	}

	if err != nil {
		log.Printf("[collector] fetch error: %v", err)
		return
	}

	if len(records) == 0 {
		log.Printf("[collector] no records returned, skipping insert")
		return
	}

	if result := c.db.Create(&records); result.Error != nil {
		log.Printf("[collector] db insert error: %v", result.Error)
		return
	}

	for _, r := range records {
		log.Printf("[collector] saved %s = $%.4f (%s)", r.Symbol, r.PriceUSD, r.Source)
	}
}

// ---------------------------------------------------------------------------
// Binance
// ---------------------------------------------------------------------------

type binanceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

func (c *Collector) fetchBinance() ([]model.Price, error) {
	// Build JSON array: ["BTCUSDT","ETHUSDT",...]
	pairs := make([]string, 0, len(c.coins))
	pairToSymbol := make(map[string]string, len(c.coins))
	for _, coin := range c.coins {
		pairs = append(pairs, `"`+coin.BinancePair+`"`)
		pairToSymbol[coin.BinancePair] = coin.Symbol
	}
	url := `https://api.binance.com/api/v3/ticker/price?symbols=[` + strings.Join(pairs, ",") + `]`

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("binance GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance HTTP %d", resp.StatusCode)
	}

	var tickers []binanceTicker
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return nil, fmt.Errorf("binance decode: %w", err)
	}

	now := time.Now().UTC()
	records := make([]model.Price, 0, len(tickers))
	for _, t := range tickers {
		sym, ok := pairToSymbol[t.Symbol]
		if !ok {
			continue
		}
		var price float64
		if _, err := fmt.Sscanf(t.Price, "%f", &price); err != nil {
			log.Printf("[collector] binance: cannot parse price %q for %s", t.Price, sym)
			continue
		}
		records = append(records, model.Price{
			Symbol:    sym,
			PriceUSD:  price,
			Source:    "binance",
			CreatedAt: now,
		})
	}
	return records, nil
}

// ---------------------------------------------------------------------------
// CoinGecko
// ---------------------------------------------------------------------------

// coinGeckoResponse: geckoID → { "usd": 67432.12 }
type coinGeckoResponse map[string]map[string]float64

func (c *Collector) fetchCoinGecko() ([]model.Price, error) {
	// Build ids param: "bitcoin,ethereum,solana,..."
	geckoIDs := make([]string, 0, len(c.coins))
	// FIX: removed unused idToSymbol map (was a compile error in previous version)
	for _, coin := range c.coins {
		geckoIDs = append(geckoIDs, coin.GeckoID)
	}
	url := "https://api.coingecko.com/api/v3/simple/price?ids=" +
		strings.Join(geckoIDs, ",") + "&vs_currencies=usd"

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("coingecko GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko HTTP %d", resp.StatusCode)
	}

	var data coinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("coingecko decode: %w", err)
	}

	now := time.Now().UTC()
	records := make([]model.Price, 0, len(c.coins))
	// Iterate c.coins (not the map) to preserve order and use coin.GeckoID correctly
	for _, coin := range c.coins {
		prices, ok := data[coin.GeckoID]
		if !ok {
			log.Printf("[collector] coingecko: no data for %s (%s)", coin.Symbol, coin.GeckoID)
			continue
		}
		usd, ok := prices["usd"]
		if !ok {
			log.Printf("[collector] coingecko: no USD price for %s", coin.Symbol)
			continue
		}
		records = append(records, model.Price{
			Symbol:    coin.Symbol,
			PriceUSD:  usd,
			Source:    "coingecko",
			CreatedAt: now,
		})
	}
	return records, nil
}
