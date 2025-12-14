#!/bin/bash
set -e

echo "Initializing FindYourRoot database..."

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Enable UUID extension
    CREATE EXTENSION IF NOT EXISTS "pgcrypto";
    
    -- Grant all privileges
    GRANT ALL PRIVILEGES ON DATABASE $POSTGRES_DB TO $POSTGRES_USER;
    
    -- Log completion
    SELECT 'Database initialization completed successfully' AS status;
EOSQL

echo "Database ready!"
