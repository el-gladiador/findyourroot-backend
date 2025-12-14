package database

import (
	"context"
	"database/sql"
	"os"

	"cloud.google.com/go/firestore"
)

type DBType string

const (
	PostgreSQL DBType = "postgres"
	Firestore  DBType = "firestore"
)

type Database struct {
	Type            DBType
	PostgresClient  *sql.DB
	FirestoreClient *firestore.Client
}

// InitDatabase initializes the appropriate database based on DB_TYPE env var
func InitDatabase(ctx context.Context) (*Database, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "postgres" // Default to PostgreSQL for local dev
	}

	db := &Database{
		Type: DBType(dbType),
	}

	switch db.Type {
	case Firestore:
		client, err := InitFirestore(ctx)
		if err != nil {
			return nil, err
		}
		db.FirestoreClient = client
	case PostgreSQL:
		fallthrough
	default:
		client, err := NewDB()
		if err != nil {
			return nil, err
		}
		if err := RunMigrations(client); err != nil {
			return nil, err
		}
		db.PostgresClient = client
	}

	return db, nil
}

// Close closes the active database connection
func (db *Database) Close() error {
	if db.FirestoreClient != nil {
		return db.FirestoreClient.Close()
	}
	if db.PostgresClient != nil {
		return db.PostgresClient.Close()
	}
	return nil
}
