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
iter := client.Collection("users").Documents(ctx)
for {
doc, err := iter.Next()
if err == iterator.Done { break }
if err != nil { log.Fatal(err) }
client.Collection("users").Doc(doc.Ref.ID).Delete(ctx)
log.Printf("Deleted user: %s", doc.Ref.ID)
}
}
