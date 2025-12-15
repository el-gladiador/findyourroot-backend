package main
import (
"context"
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
client, _ := firestore.NewClientWithDatabase(ctx, os.Getenv("GCP_PROJECT_ID"), "findyourroots-db", option.WithCredentialsFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
defer client.Close()

// Find admin user by email
iter := client.Collection("users").Where("email", "==", "mohammadamiri.py@gmail.com").Documents(ctx)
doc, err := iter.Next()
if err == iterator.Done {
log.Fatal("Admin user not found")
}
if err != nil {
log.Fatal(err)
}

// Update tree_name
_, err = doc.Ref.Update(ctx, []firestore.Update{
{Path: "tree_name", Value: "Batur"},
})
if err != nil {
log.Fatal(err)
}

log.Println("Admin user updated with tree_name: Batur")
}
