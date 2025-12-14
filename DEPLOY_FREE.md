# Deploy to Cloud Run (Free Tier) with Neon PostgreSQL

This guide uses Google Cloud Run free tier + Neon free PostgreSQL database.

## Why This Setup?

- **Cloud Run Free Tier**: 2 million requests/month, 360k GB-seconds/month
- **Neon Free Tier**: 500MB PostgreSQL database, no credit card needed
- **Total Cost**: $0/month for low traffic

## Step 1: Create Free Neon Database

1. Go to https://neon.tech
2. Sign up (free, no credit card)
3. Create a new project: "findyourroot"
4. Copy your connection string:
   ```
   postgresql://user:password@ep-xxx.region.aws.neon.tech/neondb?sslmode=require
   ```

## Step 2: Set Up Google Cloud

```bash
# Login to Google Cloud
gcloud auth login

# Create new project (or use existing)
export PROJECT_ID="findyourroot-$(date +%s)"
gcloud projects create $PROJECT_ID
gcloud config set project $PROJECT_ID

# Enable billing (required, but stays in free tier)
# Go to: https://console.cloud.google.com/billing

# Enable Cloud Run API
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

## Step 3: Set Environment Variables

Parse your Neon connection string:
```
postgresql://user:password@host:5432/dbname?sslmode=require
```

```bash
export PROJECT_ID="your-project-id"
export NEON_HOST="ep-xxx.region.aws.neon.tech"
export NEON_USER="your-user"
export NEON_PASSWORD="your-password"
export NEON_DB="neondb"
export JWT_SECRET="$(openssl rand -base64 32)"
export ADMIN_EMAIL="mohammadamiri.py@gmail.com"
export ADMIN_PASSWORD="Klgzu7.RpoG!"
export FRONTEND_URL="*"  # Allow all origins for now
```

## Step 4: Deploy Backend to Cloud Run

```bash
cd backend

# Build and push image
docker build -t gcr.io/$PROJECT_ID/findyourroot-backend:latest .
gcloud auth configure-docker
docker push gcr.io/$PROJECT_ID/findyourroot-backend:latest

# Deploy to Cloud Run (free tier settings)
gcloud run deploy findyourroot-backend \
  --image gcr.io/$PROJECT_ID/findyourroot-backend:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars "JWT_SECRET=$JWT_SECRET,DB_HOST=$NEON_HOST,DB_PORT=5432,DB_USER=$NEON_USER,DB_PASSWORD=$NEON_PASSWORD,DB_NAME=$NEON_DB,DB_SSLMODE=require,ADMIN_EMAIL=$ADMIN_EMAIL,ADMIN_PASSWORD=$ADMIN_PASSWORD,FRONTEND_URL=$FRONTEND_URL" \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 1 \
  --concurrency 80 \
  --timeout 60 \
  --project $PROJECT_ID

# Get your backend URL
gcloud run services describe findyourroot-backend \
  --platform managed \
  --region us-central1 \
  --format 'value(status.url)' \
  --project $PROJECT_ID
```

## Step 5: Deploy Frontend (Free)

### Option A: Vercel (Recommended)

1. Push code to GitHub
2. Go to https://vercel.com
3. Import repository
4. Set environment variable:
   ```
   NEXT_PUBLIC_API_URL=https://your-backend-url.run.app
   ```
5. Deploy (automatic)

### Option B: Netlify

1. Push code to GitHub
2. Go to https://netlify.com
3. Import repository
4. Build settings:
   - Build command: `npm run build`
   - Publish directory: `.next`
5. Environment variable:
   ```
   NEXT_PUBLIC_API_URL=https://your-backend-url.run.app
   ```
6. Deploy

## Step 6: Test Everything

```bash
# Test backend health
curl https://your-backend-url.run.app/health

# Test login
curl -X POST https://your-backend-url.run.app/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"mohammadamiri.py@gmail.com","password":"Klgzu7.RpoG!"}'
```

## Free Tier Limits

### Cloud Run
- ✅ 2 million requests/month
- ✅ 360k GB-seconds/month (compute time)
- ✅ 180k vCPU-seconds/month
- ✅ 1 GB network egress/month

### Neon PostgreSQL
- ✅ 500 MB storage
- ✅ 1 project
- ✅ 10 branches
- ❌ 7-day point-in-time restore

### Vercel/Netlify
- ✅ Unlimited bandwidth
- ✅ Automatic SSL
- ✅ Global CDN

## Staying in Free Tier

1. **Set min-instances=0**: Backend sleeps when not in use (cold start ~2s)
2. **Low memory (256Mi)**: Reduces compute costs
3. **Max instances=1**: Prevents scaling costs
4. **Use Neon instead of Cloud SQL**: Saves $7-10/month

## Cost Monitoring

```bash
# Check Cloud Run usage
gcloud run services describe findyourroot-backend \
  --region us-central1 \
  --format="value(status.traffic)"

# View billing
gcloud billing accounts list
```

## Alternative: Railway (All-in-One Free)

If Cloud Run seems complex, try Railway:

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login and deploy
railway login
cd backend
railway init
railway up

# Deploy frontend
cd ../frontend
railway init
railway up
```

Railway gives $5 free credit/month - enough for both backend + database.

## Troubleshooting

### Backend won't start
```bash
# Check logs
gcloud run logs read findyourroot-backend --limit 50
```

### Database connection fails
- Verify `DB_SSLMODE=require` for Neon
- Check Neon dashboard for connection string
- Test connection locally first

### Out of free tier
- Check: https://console.cloud.google.com/billing/
- Review Cloud Run metrics
- Reduce max-instances or memory

## Summary

**Total Monthly Cost**: $0
- Cloud Run: Free tier
- Neon DB: Free tier  
- Vercel/Netlify: Free tier

Perfect for personal projects and low-traffic apps!
