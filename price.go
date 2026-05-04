package model

import (
	"time"
)

// Price stores a single price snapshot for one cryptocurrency symbol.
type Price struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"   json:"id"`
	Symbol    string    `gorm:"size:16;not null;index"     json:"symbol"`   // e.g. "BTC", "ETH"
	PriceUSD  float64   `gorm:"not null"                   json:"price_usd"` // price at collection time
	Source    string    `gorm:"size:32;not null"           json:"source"`   // "binance" | "coingecko"
	CreatedAt time.Time `gorm:"autoCreateTime"             json:"created_at"`
}

// TableName overrides the default table name used by GORM.
func (Price) TableName() string {
	return "prices"
}
