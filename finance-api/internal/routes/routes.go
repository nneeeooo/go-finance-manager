package routes

import (
	"finance-api/internal/config"
	"finance-api/internal/handlers"
	"finance-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Permissions: everything below /auth and /webhooks needs a logged-in user (token checked, user loaded fresh from
// the DB for the role). Regular users reach only their own data; admin routes also need the admin role.
func Register(r *gin.Engine, cfg *config.Config) {
	api := r.Group("/api/v1")
	authed := []gin.HandlerFunc{middleware.AuthMiddleware(cfg), middleware.LoadUser()}
	admin := middleware.RequireAdmin()

	// Auth
	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/refresh", handlers.Refresh)
		auth.POST("/logout", handlers.Logout)
	}

	// Users: yourself, or anyone as an admin
	users := api.Group("/users", authed...)
	{
		users.GET("/me", handlers.GetMe)
		users.GET("", admin, handlers.ListUsers)
		users.POST("", admin, handlers.CreateUser)
		users.GET("/:id", handlers.GetUser)
		users.PATCH("/:id", handlers.UpdateUser)
		users.DELETE("/:id", admin, handlers.DeleteUser)
	}

	// Accounts: the user's own
	accounts := api.Group("/accounts", authed...)
	{
		accounts.POST("", handlers.CreateAccount)
		accounts.GET("", handlers.GetUserAccounts)
		accounts.GET("/:id", handlers.GetAccount)
		accounts.PATCH("/:id", handlers.UpdateAccount)
		accounts.DELETE("/:id", handlers.DeleteAccount)

		accounts.GET("/:id/transactions", handlers.GetAccountTransactions)
	}

	// Transactions: of the user's own accounts
	transactions := api.Group("/transactions", authed...)
	{
		transactions.GET("", handlers.GetUserTransactions)
		transactions.POST("", handlers.CreateTransaction)
		transactions.GET("/:id", handlers.GetTransaction)
		transactions.PATCH("/:id", handlers.UpdateTransaction)
		transactions.DELETE("/:id", handlers.DeleteTransaction)
	}

	// Imports: bank statements into the user's own accounts
	imports := api.Group("/imports", authed...)
	{
		imports.POST("/privatbank", handlers.ImportPrivatbank)
		imports.POST("/monobank", handlers.StartMonobankImport)
		imports.GET("/monobank", handlers.MonobankImportStatus)
		imports.DELETE("/monobank", handlers.CancelMonobankImport)
		imports.POST("/monobank/accounts", handlers.ListMonobankAccounts)
	}

	// Admin: who may register and with which role
	adminGroup := api.Group("/admin", append(authed, admin)...)
	{
		adminGroup.GET("/allowed-emails", handlers.ListAllowedEmails)
		adminGroup.POST("/allowed-emails", handlers.AddAllowedEmail)
		adminGroup.PATCH("/allowed-emails/:id", handlers.UpdateAllowedEmail)
		adminGroup.DELETE("/allowed-emails/:id", handlers.RemoveAllowedEmail)
	}

	// Webhooks (secret checked in the handler)
	webhooks := api.Group("/webhooks")
	{
		webhooks.GET("/monobank", handlers.MonobankWebhookCheck)
		webhooks.POST("/monobank", handlers.MonobankWebhookReceive)
	}
}
