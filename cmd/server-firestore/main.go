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

	// Initialize Firestore
	client, err := database.InitFirestore(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Firestore: %v", err)
	}
	defer client.Close()

	// Initialize handlers
	authHandler := handlers.NewFirestoreAuthHandler(client)
	treeHandler := handlers.NewFirestoreTreeHandler(client)

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
		}

		// Semi-protected routes (requires valid token)
		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.AuthMiddleware())
		{
			authProtected.GET("/validate", authHandler.ValidateToken)
		}

		// Protected routes (admin only)
		protected := v1.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// Tree management
			tree := protected.Group("/tree")
			{
				tree.GET("", treeHandler.GetAllPeople)
				tree.GET("/:id", treeHandler.GetPerson)
				tree.POST("", treeHandler.CreatePerson)
				tree.PUT("/:id", treeHandler.UpdatePerson)
				tree.DELETE("/:id", treeHandler.DeletePerson)
			}
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
