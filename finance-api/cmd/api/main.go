// @title Finance API
// @version 1.0
// @description API for managing personal finances.
// @host localhost:8080
// @BasePath /api/v1
package main

import (
	"errors"
	"finance-api/internal/config"
	"finance-api/internal/database"
	"finance-api/internal/models"
	"finance-api/internal/routes"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	redoc "github.com/mvrilo/go-redoc"
	ginredoc "github.com/mvrilo/go-redoc/gin"

	_ "finance-api/docs"
)

func main() {
	cfg := config.Load("../.env")
	db := database.Connect(cfg)

	if err := db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Transaction{},
		&models.RefreshTokenFamily{},
		&models.RefreshToken{},
		&models.AllowedEmail{},
	); err != nil {
		panic(err)
	}

	// Seed initial allowed emails if configured
	if initialEmails := os.Getenv("ALLOWED_REGISTRATION_EMAILS"); initialEmails != "" {
		for _, raw := range strings.Split(initialEmails, ",") {
			email := strings.ToLower(strings.TrimSpace(raw))
			if email != "" {
				var existing models.AllowedEmail
				if err := db.Where("LOWER(email) = ?", email).First(&existing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					_ = db.Create(&models.AllowedEmail{
						Email: email,
					})
				}
			}
		}
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	if cfg.GinMode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", cfg)

		c.Next()
	})

	routes.Register(r, cfg)

	doc := redoc.Redoc{
		Title:       "Finance API",
		Description: "API for managing personal finances.",
		SpecFile:    "./docs/swagger.json",
		SpecPath:    "/docs/openapi.json",
		DocsPath:    "/docs",
	}

	r.Use(ginredoc.New(doc))

	if err := r.Run(":" + cfg.AppPort); err != nil {
		panic(err)
	}
}
