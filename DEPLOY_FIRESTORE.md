## Deploy to Cloud Run with Firestore (100% Free Tier)

This setup uses:
- **Firestore**: Free tier (1GB storage, 50k reads/day, 20k writes/day)
- **Cloud Run**: Free tier (2M requests/month, 360k GB-seconds)
- **Total Cost**: $0/month for low traffic apps

---

## Step 1: Set Up Google Cloud Project

```bash
# Login
gcloud auth login

# Create project (or use existing)
export PROJECT_ID="findyourroot-app"
gcloud projects create $PROJECT_ID
gcloud config set project $PROJECT_ID

# Enable required APIs
gcloud services enable firestore.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable cloudbuild.googleapis.com
```

## Step 2: Create Firestore Database

```bash
# Create Firestore database in Native mode
gcloud firestore databases create --location=us-central1

# Or use the console:
# 1. Go to: https://console.cloud.google.com/firestore
# 2. Click "Select Native Mode"
# 3. Choose location: us-central1
# 4. Click "Create Database"
```

## Step 3: Set Environment Variables

```bash
export PROJECT_ID="your-project-id"
export GCP_PROJECT_ID=$PROJECT_ID
export JWT_SECRET="$(openssl rand -base64 48)"
export ADMIN_EMAIL="mohammadamiri.py@gmail.com"
export ADMIN_PASSWORD="Klgzu7.RpoG!"
export REGION="us-central1"
```

## Step 4: Update Dockerfile for Firestore

Create `backend/Dockerfile.firestore`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build server with Firestore
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server-firestore

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
```

## Step 5: Build and Deploy

```bash
cd backend

# Update go dependencies
go get cloud.google.com/go/firestore@latest
go get google.golang.org/api@latest
go mod tidy

# Build and deploy to Cloud Run
gcloud run deploy findyourroot-backend \
  --source . \
  --dockerfile Dockerfile.firestore \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --set-env-vars "GCP_PROJECT_ID=$PROJECT_ID,JWT_SECRET=$JWT_SECRET,ADMIN_EMAIL=$ADMIN_EMAIL,ADMIN_PASSWORD=$ADMIN_PASSWORD,FRONTEND_URL=*" \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 1 \
  --concurrency 80 \
  --timeout 60 \
  --project $PROJECT_ID
```

## Step 6: Set Up Admin User

```bash
# Get service URL
export SERVICE_URL=$(gcloud run services describe findyourroot-backend \
  --platform managed \
  --region $REGION \
  --format 'value(status.url)' \
  --project $PROJECT_ID)

# Run admin setup locally
cd backend
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"
go run cmd/setup-admin-firestore/main.go
```

### OR Setup Admin via Cloud Run Job:

```bash
gcloud run jobs create setup-admin \
  --source . \
  --dockerfile Dockerfile.firestore \
  --region $REGION \
  --set-env-vars "GCP_PROJECT_ID=$PROJECT_ID,ADMIN_EMAIL=$ADMIN_EMAIL,ADMIN_PASSWORD=$ADMIN_PASSWORD" \
  --project $PROJECT_ID \
  --command="go" \
  --args="run,./cmd/setup-admin-firestore/main.go"

# Execute the job
gcloud run jobs execute setup-admin --region $REGION
```

## Step 7: Get Your Backend URL

```bash
echo "Backend URL: $SERVICE_URL"

# Test it
curl $SERVICE_URL/health
```

## Step 8: Deploy Frontend

```bash
cd ../frontend

# Update .env.local
echo "NEXT_PUBLIC_API_URL=$SERVICE_URL" > .env.local

# Deploy to Vercel
vercel --prod

# OR Deploy to Netlify
netlify deploy --prod
```

## Free Tier Limits

### Firestore (per day)
- ✅ 50,000 document reads
- ✅ 20,000 document writes  
- ✅ 20,000 document deletes
- ✅ 1 GB storage
- ✅ 10 GB/month network egress

### Cloud Run (per month)
- ✅ 2 million requests
- ✅ 360,000 GB-seconds compute
- ✅ 180,000 vCPU-seconds
- ✅ 1 GB network egress

### Estimate for Your App
- **100 users/month**: ~10k reads, 1k writes = FREE ✅
- **1000 users/month**: ~100k reads, 10k writes = ~$0.20/month
- **Backend requests**: < 2M/month = FREE ✅

## Alternative: One-Command Deploy

Create `backend/.env.production`:

```bash
PROJECT_ID=your-project-id
GCP_PROJECT_ID=your-project-id
REGION=us-central1
JWT_SECRET=your-generated-secret
ADMIN_EMAIL=mohammadamiri.py@gmail.com
ADMIN_PASSWORD=Klgzu7.RpoG!
FRONTEND_URL=*
```

Create `backend/deploy-firestore.sh`:

```bash
#!/bin/bash
set -e

source .env.production

echo "🚀 Deploying to Cloud Run with Firestore..."

# Update dependencies
go mod tidy

# Deploy
gcloud run deploy findyourroot-backend \
  --source . \
  --dockerfile Dockerfile.firestore \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --set-env-vars "GCP_PROJECT_ID=$GCP_PROJECT_ID,JWT_SECRET=$JWT_SECRET,ADMIN_EMAIL=$ADMIN_EMAIL,ADMIN_PASSWORD=$ADMIN_PASSWORD,FRONTEND_URL=$FRONTEND_URL" \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 1 \
  --project $PROJECT_ID

echo "✅ Deployment complete!"
gcloud run services describe findyourroot-backend \
  --platform managed \
  --region $REGION \
  --format 'value(status.url)' \
  --project $PROJECT_ID
```

Then run:
```bash
chmod +x deploy-firestore.sh
./deploy-firestore.sh
```

## Troubleshooting

### Check logs
```bash
gcloud run logs read findyourroot-backend --limit 50
```

### Firestore permissions
Cloud Run automatically has access to Firestore in the same project.

### Test locally
```bash
# Download service account key
gcloud iam service-accounts keys create key.json \
  --iam-account=$PROJECT_ID@appspot.gserviceaccount.com

export GOOGLE_APPLICATION_CREDENTIALS="./key.json"
export GCP_PROJECT_ID=$PROJECT_ID

go run cmd/server-firestore/main.go
```

## Summary

✅ **100% Free for low traffic**
✅ **No database container needed**  
✅ **Automatic scaling**
✅ **No cold start for Firestore**
✅ **Global distribution**

Perfect for your family tree app! 🌳
