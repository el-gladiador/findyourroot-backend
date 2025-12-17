package main

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/firestore"
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

	email := "contributor@test.com"
	password := "test123456"

	// Check if user already exists
	iter := client.Collection("users").Where("email", "==", email).Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err == nil {
		// User exists, update role to contributor
		_, err = client.Collection("users").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "role", Value: string(models.RoleContributor)},
		})
		if err != nil {
			log.Fatalf("Failed to update user role: %v", err)
		}
		log.Printf("Updated existing user %s to contributor role", email)
		return
	}
	if err != iterator.Done {
		log.Printf("Note: %v", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create contributor user
	userID := uuid.New().String()
	user := models.User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         models.RoleContributor,
		IsAdmin:      false,
		TreeName:     "Batur",
		IsVerified:   true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = client.Collection("users").Doc(userID).Set(ctx, user)
	if err != nil {
		log.Fatalf("Failed to create contributor user: %v", err)
	}

	log.Printf("Contributor user created:")
	log.Printf("  Email: %s", email)
	log.Printf("  Password: %s", password)
	log.Printf("  Role: contributor")
	log.Println("")
	log.Println("You can now login as this user and try to add/edit family members.")
	log.Println("Their changes will be submitted as suggestions for admin approval.")
}
