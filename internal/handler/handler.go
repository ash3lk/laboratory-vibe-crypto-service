package handler

import (
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"example.com/crypto-collector/internal/model"
	"gorm.io/gorm"
)

// Triggerable is the subset of collector.Collector used by the handler.
// Using an interface keeps the handler package free of an import cycle.
type Triggerable interface {
	TriggerNow()
}

// Handler groups dependencies for the HTTP layer.
type Handler struct {
	db        *gorm.DB
	col       Triggerable
	startedAt time.Time
}

// New wires all routes and returns the handler.
//
//	staticFS — an fs.FS rooted at the web/ directory (embedded or on-disk).
func New(db *gorm.DB, col Triggerable, r *gin.Engine, staticFS fs.FS) *Handler {
	h := &Handler{
		db:        db,
		col:       col,
		startedAt: time.Now().UTC(),
	}

	// ── Frontend ─────────────────────────────────────────────────────────────
	// Serve index.html at "/" and any other static assets.
	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// ── API ───────────────────────────────────────────────────────────────────
	r.GET("/status",  h.Status)
	r.GET("/prices",  h.Prices)
	r.POST("/collect", h.Collect)

	return h
}

// ---------------------------------------------------------------------------
// GET /status
// ---------------------------------------------------------------------------

type statusResponse struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	DBPing  string `json:"db_ping"`
	Version string `json:"version"`
}

func (h *Handler) Status(c *gin.Context) {
	dbStatus   := "ok"
	httpStatus := http.StatusOK

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbStatus   = "unavailable"
		httpStatus  = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, statusResponse{
		Status:  "ok",
		Uptime:  time.Since(h.startedAt).Round(time.Second).String(),
		DBPing:  dbStatus,
		Version: "1.0.0",
	})
}

// ---------------------------------------------------------------------------
// GET /prices
// ---------------------------------------------------------------------------

type pricesResponse struct {
	Count  int64         `json:"count"`
	Prices []model.Price `json:"prices"`
}

// Prices returns price records ordered newest-first.
//
// Query params:
//
//	symbol — filter by symbol, e.g. ?symbol=BTC
//	limit  — max rows (default 50, max 500)
//	offset — pagination offset (default 0)
func (h *Handler) Prices(c *gin.Context) {
	limit  := clampInt(queryInt(c, "limit", 50), 1, 500)
	offset := maxInt(queryInt(c, "offset", 0), 0)
	symbol := c.Query("symbol")

	// Build the base query (shared between Count and Find).
	// IMPORTANT: each chained method in GORM v2 returns a new *gorm.DB clone,
	// so calling tx.Count(...) does NOT modify tx for the subsequent Find.
	tx := h.db.Model(&model.Price{}).Order("created_at DESC")
	if symbol != "" {
		tx = tx.Where("symbol = ?", symbol)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []model.Price
	if err := tx.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pricesResponse{Count: total, Prices: rows})
}

// ---------------------------------------------------------------------------
// POST /collect
// ---------------------------------------------------------------------------

// Collect triggers an immediate price collection in the background worker.
func (h *Handler) Collect(c *gin.Context) {
	h.col.TriggerNow()
	c.JSON(http.StatusAccepted, gin.H{
		"message": "collection triggered",
		"status":  "accepted",
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func queryInt(c *gin.Context, key string, defaultVal int) int {
	if s := c.Query(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return defaultVal
}

func clampInt(v, lo, hi int) int {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}

// FIX: renamed from max() to maxInt() to avoid shadowing the Go 1.21+ builtin.
func maxInt(a, b int) int {
	if a > b { return a }
	return b
}
