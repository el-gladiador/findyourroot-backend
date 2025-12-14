# Deploy to Google Cloud Run

## Prerequisites

1. **Google Cloud Account** with billing enabled
2. **gcloud CLI** installed and authenticated
3. **Docker** installed locally

## Step 1: Set up Google Cloud Project

```bash
# Login to Google Cloud
gcloud auth login

# Set your project ID
export PROJECT_ID="your-project-id"
gcloud config set project $PROJECT_ID

# Enable required APIs
gcloud services enable run.googleapis.com
gcloud services enable sql-component.googleapis.com
gcloud services enable sqladmin.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

## Step 2: Create Cloud SQL Instance

```bash
# Create PostgreSQL instance
gcloud sql instances create findyourroot-db \
  --database-version=POSTGRES_16 \
  --tier=db-f1-micro \
  --region=us-central1 \
  --root-password=YOUR_SECURE_PASSWORD

# Create database
gcloud sql databases create findyourroot \
  --instance=findyourroot-db

# Create user
gcloud sql users create dbuser \
  --instance=findyourroot-db \
  --password=YOUR_DB_PASSWORD

# Get connection name
gcloud sql instances describe findyourroot-db --format="value(connectionName)"
# Output: project-id:region:instance-name
```

## Step 3: Set Environment Variables

```bash
export JWT_SECRET="your-super-secret-jwt-key-at-least-32-chars-long"
export DB_CONNECTION_NAME="project-id:region:findyourroot-db"
export DB_USER="dbuser"
export DB_PASSWORD="YOUR_DB_PASSWORD"
export DB_NAME="findyourroot"
export ADMIN_EMAIL="mohammadamiri.py@gmail.com"
export ADMIN_PASSWORD="Klgzu7.RpoG!"
export FRONTEND_URL="https://your-frontend-url.vercel.app"
```

## Step 4: Build and Push Docker Image

```bash
cd backend

# Build for Cloud Run
docker build -t gcr.io/$PROJECT_ID/findyourroot-backend:latest .

# Authenticate Docker with GCR
gcloud auth configure-docker

# Push image
docker push gcr.io/$PROJECT_ID/findyourroot-backend:latest
```

## Step 5: Deploy to Cloud Run

```bash
gcloud run deploy findyourroot-backend \
  --image gcr.io/$PROJECT_ID/findyourroot-backend:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --add-cloudsql-instances $DB_CONNECTION_NAME \
  --set-env-vars "JWT_SECRET=$JWT_SECRET,DB_HOST=/cloudsql/$DB_CONNECTION_NAME,DB_PORT=5432,DB_USER=$DB_USER,DB_PASSWORD=$DB_PASSWORD,DB_NAME=$DB_NAME,DB_SSLMODE=disable,ADMIN_EMAIL=$ADMIN_EMAIL,ADMIN_PASSWORD=$ADMIN_PASSWORD,FRONTEND_URL=$FRONTEND_URL" \
  --memory 512Mi \
  --cpu 1 \
  --max-instances 10 \
  --timeout 60
```

## Step 6: Get Backend URL

```bash
gcloud run services describe findyourroot-backend \
  --platform managed \
  --region us-central1 \
  --format 'value(status.url)'
```

Save this URL - you'll need it for the frontend!

## Step 7: Deploy Frontend

### Option A: Vercel

1. Push code to GitHub
2. Import project in Vercel
3. Set environment variable:
   ```
   NEXT_PUBLIC_API_URL=https://your-backend-url.run.app
   ```
4. Deploy

### Option B: Netlify

1. Push code to GitHub
2. Import project in Netlify
3. Set environment variable:
   ```
   NEXT_PUBLIC_API_URL=https://your-backend-url.run.app
   ```
4. Set build command: `npm run build`
5. Set publish directory: `.next`
6. Deploy

## Step 8: Update Backend CORS (if needed)

If you know your frontend URL, you can update the backend to only allow that origin:

```bash
# Redeploy with specific frontend URL
gcloud run services update findyourroot-backend \
  --update-env-vars "FRONTEND_URL=https://your-frontend.vercel.app"
```

## Cost Estimate

- **Cloud SQL** (db-f1-micro): ~$7-10/month
- **Cloud Run**: Pay per use (~$0-5/month for low traffic)
- **Total**: ~$10-15/month

## Troubleshooting

### Check logs
```bash
gcloud run logs read findyourroot-backend --limit 50
```

### Test backend
```bash
curl https://your-backend-url.run.app/health
```

### Run migrations manually
Cloud Run automatically runs migrations on startup (in main.go).

### Connect to Cloud SQL locally (for debugging)
```bash
cloud_sql_proxy -instances=$DB_CONNECTION_NAME=tcp:5432
```

## Simplified One-Command Deploy

Create a `.env.production` file:
```bash
PROJECT_ID=your-project-id
DB_CONNECTION_NAME=project-id:region:findyourroot-db
JWT_SECRET=your-secret-key
DB_USER=dbuser
DB_PASSWORD=your-db-password
DB_NAME=findyourroot
ADMIN_EMAIL=mohammadamiri.py@gmail.com
ADMIN_PASSWORD=Klgzu7.RpoG!
FRONTEND_URL=https://your-frontend.vercel.app
```

Then run:
```bash
source .env.production
./deploy.sh
```
