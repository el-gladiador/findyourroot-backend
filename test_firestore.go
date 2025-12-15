package main

import (
"context"
"fmt"
"log"
"os"

"cloud.google.com/go/firestore"
"github.com/joho/godotenv"
"google.golang.org/api/iterator"
"google.golang.org/api/option"
)

func main() {
godotenv.Load()

ctx := context.Background()
projectID := os.Getenv("GCP_PROJECT_ID")
credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

client, err := firestore.NewClientWithDatabase(ctx, projectID, "findyourroots-db", option.WithCredentialsFile(credPath))
if err != nil {
log.Fatal(err)
}
defer client.Close()

iter := client.Collection("users").Documents(ctx)
count := 0
for {
doc, err := iter.Next()
if err == iterator.Done {
break
}
if err != nil {
log.Fatal(err)
}

data := doc.Data()
fmt.Printf("User %d: %s (ID: %s)\n", count+1, data["email"], doc.Ref.ID)
count++
}

fmt.Printf("\nTotal users: %d\n", count)
}
