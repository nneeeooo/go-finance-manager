package database

import (
	"fmt"
	"log"

	"finance-api/internal/config"
	"finance-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.PostgresHost,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresDB,
		cfg.PostgresPort,
		cfg.PostgresSSLMode,
	)

	gormConfig := &gorm.Config{}
	if cfg.GinMode == "test" {
		gormConfig.Logger = logger.Discard
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	if cfg.GinMode != "test" {
		log.Println("Database connected")
	}

	return db
}

// Migrate creates or updates every table (used by the server and the tests)
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Transaction{},
		&models.RefreshTokenFamily{},
		&models.RefreshToken{},
		&models.AllowedEmail{},
		&models.Session{},
	)
}
