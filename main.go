package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"

	"github.com/yourorg/crypto-collector/internal/collector"
	"github.com/yourorg/crypto-collector/internal/config"
	"github.com/yourorg/crypto-collector/internal/db"
	"github.com/yourorg/crypto-collector/internal/handler"
)

// webFS bundles the frontend into the binary at compile time.
//
//go:embed web
var webFS embed.FS

func main() {
	// 1. Config
	cfg := config.Load()

	// 2. Database
	database, err := db.Connect(cfg.DBURL)
	if err != nil {
		log.Fatalf("[main] failed to connect to database: %v", err)
	}

	// 3. Collector worker
	// FIX: was missing 4th argument cfg.Coins → compile error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	col := collector.New(database, cfg.PriceSource, cfg.CollectIntervalSec, cfg.Coins)
	col.Start(ctx)

	// 4. HTTP router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Strip "web/" prefix so index.html lives at "/"
	staticRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("[main] embed FS sub failed: %v", err)
	}

	handler.New(database, col, router, staticRoot)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Run
	go func() {
		log.Printf("[main] listening on http://localhost%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	// 6. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] shutting down...")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("[main] forced shutdown: %v", err)
	}
	if sqlDB, err := database.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Println("[main] exited cleanly")
}
