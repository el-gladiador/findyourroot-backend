#!/bin/bash

# Local development startup script

set -e

echo "🔧 Starting FindYourRoot Backend..."

# Check if .env exists
if [ ! -f .env ]; then
    echo "📝 Creating .env from .env.example..."
    cp .env.example .env
    echo "⚠️  Please edit .env with your configuration before continuing!"
    exit 1
fi

# Detect docker-compose or docker compose
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
elif docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    echo "❌ Docker Compose not found. Please install Docker and Docker Compose."
    exit 1
fi

# Start PostgreSQL
echo "🐘 Starting PostgreSQL..."
$DOCKER_COMPOSE up -d postgres

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
sleep 5

# Run migrations and setup by starting server in background temporarily
echo "🗄️  Running migrations..."
timeout 10s go run cmd/server/main.go > /dev/null 2>&1 || true
sleep 2

# Run admin setup
echo "👤 Setting up admin user..."
go run cmd/setup-admin/main.go

# Start the server
echo "🚀 Starting server..."
go run cmd/server/main.go
