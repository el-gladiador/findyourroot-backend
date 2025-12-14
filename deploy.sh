#!/bin/bash

# Deploy script for Google Cloud Run with Artifact Registry and Firestore

set -e

# Configuration
SERVICE_NAME="${SERVICE_NAME:-findyourroot-backend}"
REGION="${REGION:-us-central1}"
PROJECT_ID="${PROJECT_ID}"

# Artifact Registry configuration
REPOSITORY_NAME="${REPOSITORY_NAME:-findyourroot}"
ARTIFACT_REGISTRY_LOCATION="${ARTIFACT_REGISTRY_LOCATION:-us-central1}"
IMAGE_NAME="${ARTIFACT_REGISTRY_LOCATION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY_NAME}/${SERVICE_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check for required environment variables
check_required_vars() {
  local missing_vars=()
  
  if [ -z "$PROJECT_ID" ]; then
    missing_vars+=("PROJECT_ID")
  fi
  
  if [ -z "$JWT_SECRET" ]; then
    missing_vars+=("JWT_SECRET")
  fi
  
  if [ -z "$ADMIN_EMAIL" ]; then
    missing_vars+=("ADMIN_EMAIL")
  fi
  
  if [ -z "$ADMIN_PASSWORD" ]; then
    missing_vars+=("ADMIN_PASSWORD")
  fi
  
  if [ ${#missing_vars[@]} -ne 0 ]; then
    echo -e "${RED}❌ Missing required environment variables:${NC}"
    for var in "${missing_vars[@]}"; do
      echo -e "   ${RED}• $var${NC}"
    done
    echo ""
    echo "Please set them before running this script:"
    echo "  export PROJECT_ID=your-project-id"
    echo "  export JWT_SECRET=your-secret-key"
    echo "  export ADMIN_EMAIL=admin@example.com"
    echo "  export ADMIN_PASSWORD=your-password"
    exit 1
  fi
}

# Create Artifact Registry repository if it doesn't exist
setup_artifact_registry() {
  echo -e "${BLUE}📦 Setting up Artifact Registry...${NC}"
  
  # Check if repository exists
  if gcloud artifacts repositories describe ${REPOSITORY_NAME} \
    --location=${ARTIFACT_REGISTRY_LOCATION} \
    --project=${PROJECT_ID} &>/dev/null; then
    echo -e "${GREEN}✓ Repository '${REPOSITORY_NAME}' already exists${NC}"
  else
    echo -e "${YELLOW}Creating new repository '${REPOSITORY_NAME}'...${NC}"
    gcloud artifacts repositories create ${REPOSITORY_NAME} \
      --repository-format=docker \
      --location=${ARTIFACT_REGISTRY_LOCATION} \
      --description="FindYourRoot Docker images" \
      --project=${PROJECT_ID}
    echo -e "${GREEN}✓ Repository created${NC}"
  fi
}

# Apply cleanup policy to Artifact Registry
apply_cleanup_policy() {
  echo -e "${BLUE}🧹 Applying cleanup policy...${NC}"
  
  # Check if cleanup-policy.json exists
  if [ ! -f "cleanup-policy.json" ]; then
    echo -e "${YELLOW}⚠️  cleanup-policy.json not found, skipping cleanup policy setup${NC}"
    return
  fi
  
  gcloud artifacts repositories set-cleanup-policies ${REPOSITORY_NAME} \
    --location=${ARTIFACT_REGISTRY_LOCATION} \
    --policy=cleanup-policy.json \
    --project=${PROJECT_ID} || true
  
  echo -e "${GREEN}✓ Cleanup policy applied${NC}"
}

# Configure Docker authentication for Artifact Registry
configure_docker_auth() {
  echo -e "${BLUE}🔐 Configuring Docker authentication...${NC}"
  gcloud auth configure-docker ${ARTIFACT_REGISTRY_LOCATION}-docker.pkg.dev --quiet
  echo -e "${GREEN}✓ Docker authentication configured${NC}"
}

# Build and push Docker image
build_and_push_image() {
  echo -e "${BLUE}🏗️  Building Docker image...${NC}"
  docker build -f Dockerfile.firestore -t ${IMAGE_NAME}:latest -t ${IMAGE_NAME}:$(date +%Y%m%d-%H%M%S) .
  echo -e "${GREEN}✓ Image built${NC}"
  
  echo -e "${BLUE}⬆️  Pushing image to Artifact Registry...${NC}"
  docker push ${IMAGE_NAME}:latest
  docker push ${IMAGE_NAME}:$(date +%Y%m%d-%H%M%S)
  echo -e "${GREEN}✓ Image pushed${NC}"
}

# Deploy to Cloud Run with free-tier optimized settings
deploy_to_cloud_run() {
  echo -e "${BLUE}🚀 Deploying to Cloud Run...${NC}"
  
  # Frontend URL (optional)
  FRONTEND_URL="${FRONTEND_URL:-*}"
  
  gcloud run deploy ${SERVICE_NAME} \
    --image ${IMAGE_NAME}:latest \
    --platform managed \
    --region ${REGION} \
    --allow-unauthenticated \
    --set-env-vars "DB_TYPE=firestore,JWT_SECRET=${JWT_SECRET},GCP_PROJECT_ID=${PROJECT_ID},ADMIN_EMAIL=${ADMIN_EMAIL},ADMIN_PASSWORD=${ADMIN_PASSWORD},FRONTEND_URL=${FRONTEND_URL}" \
    --memory 256Mi \
    --cpu 1 \
    --min-instances 0 \
    --max-instances 3 \
    --timeout 60 \
    --concurrency 80 \
    --project ${PROJECT_ID}
  
  echo -e "${GREEN}✓ Deployment complete${NC}"
}

# Clean up old images (keep last 3, delete older than 7 days)
cleanup_old_images() {
  echo -e "${BLUE}🧹 Cleaning up old images...${NC}"
  
  # Get all image versions (excluding 'latest' tag)
  IMAGES=$(gcloud artifacts docker images list \
    ${ARTIFACT_REGISTRY_LOCATION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY_NAME}/${SERVICE_NAME} \
    --format="value(version)" \
    --filter="NOT version:latest" \
    --sort-by="~CREATE_TIME" \
    --project=${PROJECT_ID} 2>/dev/null || echo "")
  
  if [ -z "$IMAGES" ]; then
    echo -e "${YELLOW}⚠️  No images to clean up${NC}"
    return
  fi
  
  # Skip first 3 images (keep them), delete the rest
  IMAGES_TO_DELETE=$(echo "$IMAGES" | tail -n +4)
  
  if [ -z "$IMAGES_TO_DELETE" ]; then
    echo -e "${GREEN}✓ No old images to delete (keeping last 3)${NC}"
    return
  fi
  
  echo -e "${YELLOW}Deleting old image versions...${NC}"
  while IFS= read -r version; do
    if [ ! -z "$version" ]; then
      echo -e "  ${YELLOW}• Deleting version: ${version}${NC}"
      gcloud artifacts docker images delete \
        ${ARTIFACT_REGISTRY_LOCATION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY_NAME}/${SERVICE_NAME}@${version} \
        --quiet \
        --project=${PROJECT_ID} || true
    fi
  done <<< "$IMAGES_TO_DELETE"
  
  echo -e "${GREEN}✓ Old images cleaned up${NC}"
}

# Display service information
show_service_info() {
  echo ""
  echo -e "${GREEN}✅ Deployment successful!${NC}"
  echo ""
  echo -e "${BLUE}📊 Service Information:${NC}"
  echo -e "  ${YELLOW}Service Name:${NC} ${SERVICE_NAME}"
  echo -e "  ${YELLOW}Region:${NC} ${REGION}"
  echo -e "  ${YELLOW}Project:${NC} ${PROJECT_ID}"
  echo ""
  echo -e "${BLUE}🔗 Service URL:${NC}"
  SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} \
    --platform managed \
    --region ${REGION} \
    --format 'value(status.url)' \
    --project ${PROJECT_ID})
  echo -e "  ${GREEN}${SERVICE_URL}${NC}"
  echo ""
  echo -e "${BLUE}📝 Next Steps:${NC}"
  echo "  1. Test the health endpoint: curl ${SERVICE_URL}/health"
  echo "  2. Update your frontend .env: NEXT_PUBLIC_API_URL=${SERVICE_URL}"
  echo "  3. Test the login endpoint with your admin credentials"
  echo ""
}

# Main execution
main() {
  echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
  echo -e "${GREEN}  FindYourRoot Backend - Cloud Run Deployment${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
  echo ""
  
  check_required_vars
  setup_artifact_registry
  apply_cleanup_policy
  configure_docker_auth
  build_and_push_image
  deploy_to_cloud_run
  cleanup_old_images
  show_service_info
}

# Run main function
main
