package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if sslmode == "" {
		sslmode = "disable"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Get admin credentials from environment
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" || adminPassword == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD must be set")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Check if admin user already exists
	var exists bool
	err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, adminEmail).Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check if user exists: %v", err)
	}

	if exists {
		// Update existing admin
		_, err = db.Exec(`
			UPDATE users 
			SET password_hash = $1, is_admin = true, updated_at = CURRENT_TIMESTAMP
			WHERE email = $2
		`, string(hashedPassword), adminEmail)
		if err != nil {
			log.Fatalf("Failed to update admin user: %v", err)
		}
		log.Printf("Admin user updated: %s", adminEmail)
	} else {
		// Create new admin
		_, err = db.Exec(`
			INSERT INTO users (email, password_hash, is_admin)
			VALUES ($1, $2, true)
		`, adminEmail, string(hashedPassword))
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Admin user created: %s", adminEmail)
	}

	log.Println("Admin setup completed successfully!")
}
