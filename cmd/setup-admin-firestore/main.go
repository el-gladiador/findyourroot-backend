package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mamiri/findyourroot/internal/database"
	"github.com/mamiri/findyourroot/internal/models"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/iterator"
)

func main() {
	// Load environment variables
	godotenv.Load()

	ctx := context.Background()

	// Initialize Firestore
	client, err := database.InitFirestore(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Firestore: %v", err)
	}
	defer client.Close()

	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")

	if email == "" || password == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD must be set")
	}

	// Check if user already exists
	iter := client.Collection("users").Where("email", "==", email).Limit(1).Documents(ctx)
	_, err = iter.Next()
	if err == nil {
		log.Printf("Admin user already exists: %s", email)
		return
	}
	if err != iterator.Done {
		log.Fatalf("Failed to check if user exists: %v", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	userID := uuid.New().String()
	user := models.User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         models.RoleAdmin,
		IsAdmin:      true,
		IsVerified:   true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = client.Collection("users").Doc(userID).Set(ctx, user)
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Printf("Admin user created: %s", email)
	log.Println("Admin setup completed successfully!")
}
