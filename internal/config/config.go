package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Database
	DBURL string // DB_URL — full DSN, e.g. postgres://user:pass@host:5432/dbname

	// AWS / S3 (reserved for future upload of snapshots)
	S3Bucket string // S3_BUCKET

	// Collector settings
	CollectIntervalSec int      // COLLECT_INTERVAL_SEC (default: 60)
	PriceSource        string   // PRICE_SOURCE: "binance" | "coingecko" (default: "binance")
	Coins              []string // COINS — comma-separated list, e.g. "BTC,ETH,SOL"

	// HTTP
	HTTPPort string // HTTP_PORT (default: "8080")
}

// defaultCoins is used when COINS env var is not set.
const defaultCoins = "BTC,ETH,SOL,BNB,XRP,ADA,DOGE"

// Load reads configuration from environment variables.
// It fatals if required variables are missing so the app fails fast at startup.
func Load() *Config {
	coins := parseCoins(getEnv("COINS", defaultCoins))

	cfg := &Config{
		DBURL:              requireEnv("DB_URL"),
		S3Bucket:           getEnv("S3_BUCKET", ""),
		PriceSource:        getEnv("PRICE_SOURCE", "binance"),
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		CollectIntervalSec: getEnvInt("COLLECT_INTERVAL_SEC", 60),
		Coins:              coins,
	}

	log.Printf("[config] price_source=%s interval=%ds port=%s coins=%v",
		cfg.PriceSource, cfg.CollectIntervalSec, cfg.HTTPPort, cfg.Coins)

	return cfg
}

// parseCoins splits a comma-separated string into a deduplicated, uppercased slice.
func parseCoins(raw string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		sym := strings.TrimSpace(strings.ToUpper(s))
		if sym != "" && !seen[sym] {
			seen[sym] = true
			out = append(out, sym)
		}
	}
	if len(out) == 0 {
		log.Fatal("[config] COINS is empty — specify at least one symbol, e.g. COINS=BTC,ETH")
	}
	return out
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[config] required environment variable %q is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("[config] %q must be an integer, got %q", key, v)
		}
		return n
	}
	return fallback
}
