# Cloud Run Deployment Guide

Complete guide for deploying FindYourRoot backend to Google Cloud Run with Firestore (free tier optimized).

## Quick Start

### Prerequisites
```bash
# Install gcloud CLI
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Login and set project
gcloud init
gcloud auth login

# Set your project
export PROJECT_ID=your-project-id
gcloud config set project $PROJECT_ID
```

### Enable APIs & Create Firestore
```bash
# Enable required APIs
gcloud services enable \
  cloudrun.googleapis.com \
  artifactregistry.googleapis.com \
  firestore.googleapis.com

# Create Firestore database
gcloud firestore databases create --region=us-central1
```

### Deploy Manually
```bash
cd backend

# Set required environment variables
export PROJECT_ID=your-project-id
export JWT_SECRET=$(openssl rand -base64 32)
export ADMIN_EMAIL=mohammadamiri.py@gmail.com
export ADMIN_PASSWORD=Klgzu7.RpoG!

# Run deployment script
chmod +x deploy.sh
./deploy.sh
```

## GitHub Actions CI/CD

### 1. Create Service Account
```bash
# Create service account
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions Deployer"

# Grant roles
for role in run.admin artifactregistry.admin iam.serviceAccountUser; do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:github-actions@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/$role"
done

# Create key
gcloud iam service-accounts keys create github-actions-key.json \
  --iam-account=github-actions@${PROJECT_ID}.iam.gserviceaccount.com

# Copy the key content for GitHub secrets
cat github-actions-key.json
```

### 2. Set GitHub Secrets

Go to: `Repository Settings → Secrets and variables → Actions`

Add these secrets:
- `GCP_PROJECT_ID` - Your project ID
- `GCP_SA_KEY` - Contents of `github-actions-key.json`
- `JWT_SECRET` - JWT secret key
- `ADMIN_EMAIL` - Admin email
- `ADMIN_PASSWORD` - Admin password

### 3. Deploy

Push to `main` branch - GitHub Actions will automatically deploy!

## Free Tier Optimization

### Resources (Applied Automatically)
- **Memory**: 256Mi
- **CPU**: 1 vCPU
- **Min instances**: 0 (scale to zero)
- **Max instances**: 3
- **Timeout**: 60s
- **Concurrency**: 80 requests/instance

### Free Tier Limits
- **Cloud Run**: 2M requests/month, 360K GB-seconds
- **Firestore**: 1GB storage, 50K reads/day, 20K writes/day
- **Artifact Registry**: 0.5GB storage (auto-cleanup keeps last 3 images)

### Cost: $0-2/month for typical usage! 🎉

## Troubleshooting

### View Logs
```bash
gcloud run services logs tail findyourroot-backend --region us-central1
```

### Test Deployment
```bash
# Get service URL
SERVICE_URL=$(gcloud run services describe findyourroot-backend \
  --region us-central1 --format='value(status.url)')

# Test health
curl $SERVICE_URL/health

# Test login
curl -X POST $SERVICE_URL/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your-email","password":"your-password"}'
```

### Common Issues

**Authentication errors**: Re-authenticate Docker
```bash
gcloud auth configure-docker us-central1-docker.pkg.dev
```

**Deployment fails**: Check logs
```bash
gcloud run services describe findyourroot-backend --region us-central1
```

## Database Switching

The backend uses a factory pattern that automatically detects the database type:

- **Local Development**: `DB_TYPE=postgres` (Docker PostgreSQL)
- **Production**: `DB_TYPE=firestore` (Cloud Firestore)

No code changes needed - just set the environment variable!

## Next Steps

1. Deploy frontend to Vercel/Netlify
2. Set `NEXT_PUBLIC_API_URL` to your Cloud Run URL
3. Update `FRONTEND_URL` env var for CORS
4. Set up monitoring in Cloud Console

## Resources
- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Firestore Documentation](https://cloud.google.com/firestore/docs)
- [Free Tier Details](https://cloud.google.com/free)
