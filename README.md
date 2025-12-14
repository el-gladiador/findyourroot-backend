# FindYourRoot Backend

A secure Go backend for managing family tree data with JWT authentication.

## Features

- **JWT Authentication**: Secure token-based authentication
- **Admin-only Access**: Only admin users can add/edit/delete people
- **PostgreSQL Database**: Robust data storage
- **RESTful API**: Clean API design
- **Docker Support**: Easy deployment with Docker and docker-compose
- **Cloud Run Ready**: Optimized for Google Cloud Run deployment

## Prerequisites

- Go 1.21+
- PostgreSQL 16+
- Docker & Docker Compose (optional, for containerized setup)

## Local Development Setup

1. **Clone and navigate to backend directory**
   ```bash
   cd backend
   ```

2. **Copy environment file**
   ```bash
   cp .env.example .env
   ```

3. **Update .env with your configuration**
   - Set a strong `JWT_SECRET`
   - Configure database credentials
   - Set `FRONTEND_URL` to your frontend URL

4. **Install dependencies**
   ```bash
   go mod download
   ```

5. **Start PostgreSQL** (if not using Docker)
   ```bash
   # Or use Docker Compose (recommended)
   docker-compose up -d postgres
   ```

6. **Run migrations and create admin user**
   ```bash
   go run cmd/setup-admin/main.go
   ```

7. **Start the server**
   ```bash
   go run cmd/server/main.go
   ```

The API will be available at `http://localhost:8080`

## Docker Setup

1. **Build and run with docker-compose**
   ```bash
   docker-compose up -d
   ```

2. **Create admin user**
   ```bash
   docker-compose exec backend go run cmd/setup-admin/main.go
   ```

## API Endpoints

### Authentication

**POST** `/api/v1/auth/login`
```json
{
  "email": "mohammadamiri.py@gmail.com",
  "password": "your-password"
}
```

Response:
```json
{
  "token": "eyJhbGc...",
  "user": {
    "id": "uuid",
    "email": "mohammadamiri.py@gmail.com",
    "is_admin": true
  }
}
```

### Tree Management (Protected - Requires JWT)

**GET** `/api/v1/tree` - Get all people  
**GET** `/api/v1/tree/:id` - Get person by ID  
**POST** `/api/v1/tree` - Create new person  
**PUT** `/api/v1/tree/:id` - Update person  
**DELETE** `/api/v1/tree/:id` - Delete person  

All protected endpoints require `Authorization: Bearer <token>` header.

## Cloud Run Deployment

1. **Build Docker image**
   ```bash
   docker build -t gcr.io/YOUR_PROJECT_ID/findyourroot-backend .
   ```

2. **Push to Google Container Registry**
   ```bash
   docker push gcr.io/YOUR_PROJECT_ID/findyourroot-backend
   ```

3. **Deploy to Cloud Run**
   ```bash
   gcloud run deploy findyourroot-backend \
     --image gcr.io/YOUR_PROJECT_ID/findyourroot-backend \
     --platform managed \
     --region us-central1 \
     --allow-unauthenticated \
     --set-env-vars="JWT_SECRET=your-secret,DB_HOST=your-db-host,DB_USER=your-db-user,DB_PASSWORD=your-db-password,DB_NAME=findyourroot,FRONTEND_URL=https://your-frontend.com"
   ```

4. **Set up Cloud SQL** (recommended for production)
   - Create a PostgreSQL instance in Cloud SQL
   - Use Cloud SQL Proxy or private IP for connection
   - Update DB_HOST with Cloud SQL connection name

## Security Features

- Password hashing with bcrypt
- JWT token expiration (24 hours)
- Admin-only authorization middleware
- SQL injection protection with parameterized queries
- CORS configuration
- Environment-based secrets

## Project Structure

```
backend/
├── cmd/
│   ├── server/          # Main server application
│   └── setup-admin/     # Admin user setup utility
├── internal/
│   ├── database/        # Database connection and migrations
│   ├── handlers/        # HTTP request handlers
│   ├── middleware/      # Authentication middleware
│   └── models/          # Data models
├── Dockerfile           # Docker container definition
├── docker-compose.yml   # Docker compose configuration
├── go.mod              # Go dependencies
└── .env.example        # Environment variables template
```

## Admin Credentials

Default admin user:
- Email: `mohammadamiri.py@gmail.com`
- Password: `Klgzu7.RpoG!`

**⚠️ Change these credentials in production!**

## License

Private Project
