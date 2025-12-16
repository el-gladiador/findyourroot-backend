//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	projectID := "findyourroots-481413"
	databaseID := "findyourroots-db"

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID, option.WithCredentialsFile("./serviceAccountKey.json"))
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	// supress warning
	fmt.Println("=== Firestore People Collection ===")

	iter := client.Collection("people").Documents(ctx)
	count := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error iterating: %v", err)
			break
		}

		count++
		data := doc.Data()
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Printf("Document ID: %s\n", doc.Ref.ID)
		fmt.Printf("Data: %s\n\n", jsonData)
	}

	if count == 0 {
		fmt.Println("No people found in Firestore!")
	} else {
		fmt.Printf("Total people: %d\n", count)
	}
}
