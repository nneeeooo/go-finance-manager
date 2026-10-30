package routes

import (
	"finance-api/internal/config"
	"finance-api/internal/handlers"
	"finance-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, cfg *config.Config) {
	api := r.Group("/api/v1")

	// Auth
	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/refresh", handlers.Refresh)
		auth.POST("/logout", handlers.Logout)
	}

	// Users
	users := api.Group("/users")
	{
		users.POST("", handlers.CreateUser)
		// users.GET("", handlers.GetUsers)
		users.GET("/:id", handlers.GetUser)
		// users.PATCH("/:id", handlers.UpdateUser)
		users.DELETE("/:id", handlers.DeleteUser)

	}

	// Accounts
	accounts := api.Group("/accounts")
	accounts.Use(middleware.AuthMiddleware(cfg))
	{
		accounts.POST("", handlers.CreateAccount)
		accounts.GET("", handlers.GetUserAccounts)
		accounts.GET("/:id", handlers.GetAccount)
		// accounts.PATCH("/:id", handlers.UpdateAccount)
		accounts.DELETE("/:id", handlers.DeleteAccount)

		accounts.GET("/:id/transactions", handlers.GetAccountTransactions)

	}

	// Transactions
	transactions := api.Group("/transactions")
	transactions.Use(middleware.AuthMiddleware(cfg))
	{
		transactions.GET("", handlers.GetUserTransactions)
		transactions.POST("", handlers.CreateTransaction)
		// transactions.GET("", handlers.GetTransactions)
		// transactions.GET("/:id", handlers.GetTransaction)
		transactions.DELETE("/:id", handlers.DeleteTransaction)
	}
}
