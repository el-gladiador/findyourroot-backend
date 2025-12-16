package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mamiri/findyourroot/internal/database"
	"github.com/mamiri/findyourroot/internal/handlers"
	"github.com/mamiri/findyourroot/internal/middleware"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	ctx := context.Background()

	// Check database type
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "firestore" // Default to firestore for this server
	}

	log.Printf("Using database type: %s", dbType)

	var authHandler interface {
		Login(c *gin.Context)
		Register(c *gin.Context)
		ValidateToken(c *gin.Context)
		RequestPermission(c *gin.Context)
		GetPermissionRequests(c *gin.Context)
		ApprovePermissionRequest(c *gin.Context)
		RejectPermissionRequest(c *gin.Context)
	}
	var treeHandler interface {
		GetAllPeople(c *gin.Context)
		GetPerson(c *gin.Context)
		CreatePerson(c *gin.Context)
		UpdatePerson(c *gin.Context)
		DeletePerson(c *gin.Context)
		DeleteAllPeople(c *gin.Context)
	}
	var searchHandler interface {
		SearchPeople(c *gin.Context)
		GetLocations(c *gin.Context)
		GetRoles(c *gin.Context)
	}
	var exportHandler interface {
		ExportJSON(c *gin.Context)
		ExportCSV(c *gin.Context)
		ExportText(c *gin.Context)
	}

	if dbType == "postgres" {
		// Initialize PostgreSQL
		db, err := database.NewDB()
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}
		defer db.Close()

		// Run migrations
		if err := database.RunMigrations(db); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}

		// Initialize PostgreSQL handlers
		authHandler = handlers.NewAuthHandler(db)
		treeHandler = handlers.NewTreeHandler(db)
		// Note: Search and export handlers not implemented for PostgreSQL yet
		// For now, use Firestore for full functionality
		log.Println("Warning: Search and export handlers not available for PostgreSQL")
	} else {
		// Initialize Firestore
		client, err := database.InitFirestore(ctx)
		if err != nil {
			log.Fatalf("Failed to connect to Firestore: %v", err)
		}
		defer client.Close()

		// Initialize Firestore handlers
		authHandler = handlers.NewFirestoreAuthHandler(client)
		treeHandler = handlers.NewFirestoreTreeHandler(client)
		searchHandler = handlers.NewFirestoreSearchHandler(client)
		exportHandler = handlers.NewFirestoreExportHandler(client)
	}

	// Setup Gin router
	router := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	router.Use(cors.New(config))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
		}

		// Semi-protected routes (requires valid token)
		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.AuthMiddleware())
		{
			authProtected.GET("/validate", authHandler.ValidateToken)
			authProtected.POST("/request-permission", authHandler.RequestPermission)
		}

		// Admin routes
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			admin.GET("/permission-requests", authHandler.GetPermissionRequests)
			admin.POST("/permission-requests/:id/approve", authHandler.ApprovePermissionRequest)
			admin.POST("/permission-requests/:id/reject", authHandler.RejectPermissionRequest)
		}

		// Tree routes - split by permission level
		treePublic := v1.Group("/tree")
		treePublic.Use(middleware.AuthMiddleware())
		{
			treePublic.GET("", treeHandler.GetAllPeople)
			treePublic.GET("/:id", treeHandler.GetPerson)
		}

		// Search routes (authenticated users can search)
		if searchHandler != nil {
			search := v1.Group("/search")
			search.Use(middleware.AuthMiddleware())
			{
				search.GET("", searchHandler.SearchPeople)
				search.GET("/locations", searchHandler.GetLocations)
				search.GET("/roles", searchHandler.GetRoles)
			}
		}

		// Export routes (authenticated users can export)
		if exportHandler != nil {
			export := v1.Group("/export")
			export.Use(middleware.AuthMiddleware())
			{
				export.GET("/json", exportHandler.ExportJSON)
				export.GET("/csv", exportHandler.ExportCSV)
				export.GET("/text", exportHandler.ExportText)
			}
		}

		treeEditor := v1.Group("/tree")
		treeEditor.Use(middleware.AuthMiddleware(), middleware.RequireEditor())
		{
			treeEditor.POST("", treeHandler.CreatePerson)
			treeEditor.PUT("/:id", treeHandler.UpdatePerson)
			treeEditor.DELETE("/:id", treeHandler.DeletePerson)
		}

		treeAdmin := v1.Group("/tree")
		treeAdmin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			treeAdmin.DELETE("/all", treeHandler.DeleteAllPeople)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
