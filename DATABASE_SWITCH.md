# Database Switching Guide

The backend supports **two database options** that can be switched via environment variable.

## Local Development (PostgreSQL)

Use Docker PostgreSQL for local development:

**`.env` configuration:**
```bash
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=findyourroot
```

**Start:**
```bash
cd backend
docker compose up -d
```

## Production (Firestore)

Use Firestore for production deployment:

**Environment variables:**
```bash
DB_TYPE=firestore
GCP_PROJECT_ID=your-project-id
# GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json  # Only for local testing
```

**Deploy to Cloud Run:**
```bash
gcloud run deploy findyourroot-backend \
  --source . \
  --set-env-vars "DB_TYPE=firestore,GCP_PROJECT_ID=your-project-id,..."
```

## How It Works

The backend automatically detects `DB_TYPE` and uses the appropriate database:

- `DB_TYPE=postgres` → Uses PostgreSQL (default)
- `DB_TYPE=firestore` → Uses Firestore

## Benefits

✅ **Develop locally** with PostgreSQL in Docker  
✅ **Deploy to production** with Firestore (free tier)  
✅ **No code changes** needed - just environment variables  
✅ **Same API endpoints** work with both databases  

## Quick Switch

### Local → Production
```bash
# Update .env
DB_TYPE=firestore
GCP_PROJECT_ID=your-project-id

# Restart
docker compose restart backend
```

### Production → Local
```bash
# Update .env
DB_TYPE=postgres

# Restart
docker compose restart backend
```

## Current Setup

Check which database is active:
```bash
docker logs findyourroot_backend | grep "Using database type"
```

Output:
- `Using database type: postgres` - Local PostgreSQL
- `Using database type: firestore` - Cloud Firestore
