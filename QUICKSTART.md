# Quick Start Guide

## Prerequisites Check

```bash
# Check Go installation
go version  # Should be 1.21 or higher

# Check Docker installation
docker --version
docker compose version
```

## Setup in 3 Steps

### 1. Environment Setup
```bash
cd backend

# Copy and edit .env (already done for you)
# The .env file is configured with:
# - Database: PostgreSQL on localhost:5432
# - JWT Secret: Pre-configured (change for production!)
# - Admin: mohammadamiri.py@gmail.com / Klgzu7.RpoG!
```

### 2. Start PostgreSQL Database
```bash
# Using Make (recommended)
make docker-up

# OR using Docker directly
docker compose up -d postgres

# Verify it's running
docker ps
```

### 3. Initialize and Run Server
```bash
# Setup admin user
go run cmd/setup-admin/main.go

# Start the server
go run cmd/server/main.go
```

## Verify Installation

### Test the API

1. **Health Check**
```bash
curl http://localhost:8080/health
# Expected: {"status":"healthy"}
```

2. **Login**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "mohammadamiri.py@gmail.com",
    "password": "Klgzu7.RpoG!"
  }'
```

You should get a response like:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "...",
    "email": "mohammadamiri.py@gmail.com",
    "is_admin": true
  }
}
```

3. **Use Token to Access Protected Endpoints**
```bash
# Save token from previous response
TOKEN="your-token-here"

# Get all people
curl http://localhost:8080/api/v1/tree \
  -H "Authorization: Bearer $TOKEN"
```

## Common Issues

### "PostgreSQL not ready"
Wait a few more seconds and try again:
```bash
docker logs findyourroot_db
```

### "JWT_SECRET not configured"
Make sure your `.env` file has `JWT_SECRET` set.

### "Port 8080 already in use"
Change `PORT` in `.env` or stop the conflicting service.

## Development Tips

```bash
# View logs
make docker-logs

# Stop services
make docker-down

# Clean everything (including database!)
make docker-clean

# Hot reload during development
make dev
```

## Next Steps

1. ✅ Backend is running on http://localhost:8080
2. 🔗 Connect your frontend to this backend
3. 🚀 Deploy to Cloud Run when ready

## API Documentation

**Base URL**: http://localhost:8080/api/v1

### Authentication
- `POST /auth/login` - Login and get JWT token
- `GET /auth/validate` - Validate current token (requires auth)

### Tree Management (Admin Only)
- `GET /tree` - Get all people
- `GET /tree/:id` - Get person by ID
- `POST /tree` - Create new person
- `PUT /tree/:id` - Update person
- `DELETE /tree/:id` - Delete person

All protected endpoints require:
```
Authorization: Bearer <your-jwt-token>
```
