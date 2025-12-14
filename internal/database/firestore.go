package database

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

var FirestoreClient *firestore.Client

// InitFirestore initializes Firestore client
func InitFirestore(ctx context.Context) (*firestore.Client, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		log.Fatal("GCP_PROJECT_ID environment variable is required")
	}

	// For Cloud Run, authentication is automatic
	// For local development, set GOOGLE_APPLICATION_CREDENTIALS
	var client *firestore.Client
	var err error

	credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credPath != "" {
		client, err = firestore.NewClient(ctx, projectID, option.WithCredentialsFile(credPath))
	} else {
		client, err = firestore.NewClient(ctx, projectID)
	}

	if err != nil {
		return nil, err
	}

	FirestoreClient = client
	log.Printf("Connected to Firestore in project: %s", projectID)
	return client, nil
}
