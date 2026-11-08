// @title Finance API
// @version 1.0
// @description API for managing personal finances.
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description "Bearer <access_token>" from /auth/login, /auth/register or /auth/refresh
package main

import (
	"finance-api/internal/config"
	"finance-api/internal/database"
	"finance-api/internal/routes"
	"finance-api/internal/services"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	redoc "github.com/mvrilo/go-redoc"
	ginredoc "github.com/mvrilo/go-redoc/gin"

	_ "finance-api/docs"
)

func main() {
	cfg := config.Load("../.env")
	db := database.Connect(cfg)

	if err := database.Migrate(db); err != nil {
		panic(err)
	}

	// Allow-list: the default emails (lytvyn349@gmail.com is the admin) plus ALLOWED_REGISTRATION_EMAILS
	if err := services.SeedAccess(db, os.Getenv("ALLOWED_REGISTRATION_EMAILS")); err != nil {
		panic(err)
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
			"X-Requested-With",
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
