package db

import (
	"log"
	"time"

	"github.com/yourorg/crypto-collector/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a PostgreSQL connection via GORM and runs auto-migration.
// dsn format: "host=... user=... password=... dbname=... port=5432 sslmode=require"
// or the shorthand URL: "postgres://user:pass@host:5432/dbname"
func Connect(dsn string) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	var (
		db  *gorm.DB
		err error
	)

	// Retry loop — useful when the container starts before the DB is ready.
	for attempt := 1; attempt <= 10; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), gormCfg)
		if err == nil {
			break
		}
		log.Printf("[db] connection attempt %d/10 failed: %v — retrying in 3s", attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, err
	}

	// Connection-pool tuning
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// Auto-migrate schema
	if err := db.AutoMigrate(&model.Price{}); err != nil {
		return nil, err
	}

	log.Println("[db] connected and migrated successfully")
	return db, nil
}
