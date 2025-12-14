# Deploy to Render (Free Tier with PostgreSQL)

Deploy both backend and PostgreSQL database on Render's free tier.

## Why Render?

- ✅ **Free PostgreSQL** with 1GB storage
- ✅ **Free web services** (750 hours/month)
- ✅ **Persistent storage** for database
- ✅ **Auto SSL/HTTPS**
- ✅ **No credit card** required for free tier

## Step 1: Create Render Account

1. Go to https://render.com
2. Sign up with GitHub (recommended)
3. No credit card needed

## Step 2: Create PostgreSQL Database

1. Click **"New +"** → **"PostgreSQL"**
2. Settings:
   - **Name**: `findyourroot-db`
   - **Database**: `findyourroot`
   - **User**: `findyourroot_user`
   - **Region**: Choose closest to you
   - **Plan**: **Free**
3. Click **"Create Database"**
4. Copy the **Internal Database URL** (starts with `postgresql://`)

Example: `postgresql://findyourroot_user:xxx@dpg-xxx-internal/findyourroot`

## Step 3: Push Code to GitHub

```bash
cd /home/mamiri/Projects/go/findyourroot
git init
git add .
git commit -m "Initial commit"
git branch -M main
git remote add origin https://github.com/yourusername/findyourroot.git
git push -u origin main
```

## Step 4: Deploy Backend

1. In Render Dashboard, click **"New +"** → **"Web Service"**
2. Connect your GitHub repository
3. Settings:
   - **Name**: `findyourroot-backend`
   - **Region**: Same as database
   - **Branch**: `main`
   - **Root Directory**: `backend`
   - **Runtime**: **Docker**
   - **Plan**: **Free**

4. **Environment Variables** (click "Advanced"):
   ```
   DATABASE_URL=<paste-internal-database-url>
   DB_HOST=<extract-host-from-url>
   DB_PORT=5432
   DB_USER=<extract-user-from-url>
   DB_PASSWORD=<extract-password-from-url>
   DB_NAME=findyourroot
   DB_SSLMODE=require
   JWT_SECRET=<generate-random-32-char-string>
   ADMIN_EMAIL=mohammadamiri.py@gmail.com
   ADMIN_PASSWORD=Klgzu7.RpoG!
   FRONTEND_URL=*
   PORT=8080
   ```

5. Click **"Create Web Service"**

## Step 5: Get Backend URL

After deployment completes (5-10 min):
- Your backend URL: `https://findyourroot-backend.onrender.com`
- Test: `curl https://findyourroot-backend.onrender.com/health`

## Step 6: Deploy Frontend

### Option A: Vercel (Recommended)

```bash
cd frontend
npm install -g vercel
vercel login
vercel
```

Set environment variable in Vercel dashboard:
```
NEXT_PUBLIC_API_URL=https://findyourroot-backend.onrender.com
```

### Option B: Netlify

```bash
cd frontend
npm install -g netlify-cli
netlify login
netlify deploy --prod
```

Set environment variable:
```
NEXT_PUBLIC_API_URL=https://findyourroot-backend.onrender.com
```

### Option C: Render (All in One)

1. Click **"New +"** → **"Static Site"**
2. Connect GitHub repo
3. Settings:
   - **Name**: `findyourroot-frontend`
   - **Root Directory**: `frontend`
   - **Build Command**: `npm install && npm run build`
   - **Publish Directory**: `.next`
4. Environment variable:
   ```
   NEXT_PUBLIC_API_URL=https://findyourroot-backend.onrender.com
   ```

## Free Tier Limits

### Render Free Tier Includes:
- ✅ PostgreSQL: 1GB storage, 90 days data retention
- ✅ Web Service: 750 hours/month (enough for 1 service)
- ✅ Static Sites: Unlimited
- ⚠️ **Note**: Free services sleep after 15 min inactivity (cold start ~30s)

## Keep Services Awake (Optional)

Use a free uptime monitor to ping your service every 14 minutes:

1. Go to https://uptimerobot.com (free)
2. Create monitor for: `https://findyourroot-backend.onrender.com/health`
3. Check interval: **every 5 minutes**

## Render.yaml (Alternative: Infrastructure as Code)

Create `render.yaml` in project root:

```yaml
services:
  # Backend
  - type: web
    name: findyourroot-backend
    runtime: docker
    dockerfilePath: ./backend/Dockerfile
    dockerContext: ./backend
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: findyourroot-db
          property: connectionString
      - key: DB_HOST
        fromDatabase:
          name: findyourroot-db
          property: host
      - key: DB_PORT
        value: 5432
      - key: DB_USER
        fromDatabase:
          name: findyourroot-db
          property: user
      - key: DB_PASSWORD
        fromDatabase:
          name: findyourroot-db
          property: password
      - key: DB_NAME
        fromDatabase:
          name: findyourroot-db
          property: database
      - key: DB_SSLMODE
        value: require
      - key: JWT_SECRET
        generateValue: true
      - key: ADMIN_EMAIL
        value: mohammadamiri.py@gmail.com
      - key: ADMIN_PASSWORD
        value: Klgzu7.RpoG!
      - key: FRONTEND_URL
        value: "*"
      - key: PORT
        value: 8080

  # Frontend
  - type: web
    name: findyourroot-frontend
    runtime: node
    buildCommand: cd frontend && npm install && npm run build
    startCommand: cd frontend && npm start
    envVars:
      - key: NEXT_PUBLIC_API_URL
        value: https://findyourroot-backend.onrender.com

databases:
  - name: findyourroot-db
    databaseName: findyourroot
    user: findyourroot_user
    plan: free
```

Then deploy with:
```bash
render deploy
```

## Troubleshooting

### Check logs
```bash
# In Render dashboard → Your Service → Logs
```

### Database connection issues
- Use **Internal Database URL** (not External)
- Ensure `DB_SSLMODE=require`
- Check database is in same region

### Service won't start
- Check Dockerfile builds locally first
- Review environment variables
- Check PORT=8080 is set

## Cost

**Total: $0/month**
- PostgreSQL: Free (1GB)
- Backend: Free (sleeps after 15 min)
- Frontend on Vercel/Netlify: Free

Perfect for your use case! 🎉
