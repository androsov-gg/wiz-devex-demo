#!/bin/bash
set -e

# ==============================================================================
# Wiz Demo Platform: Argolis GCP Bootstrap & Infrastructure Setup
# ==============================================================================

PROJECT_ID="${PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${REGION:-europe-west1}"

if [ -z "${PROJECT_ID}" ] || [ "${PROJECT_ID}" = "(unset)" ]; then
  echo "❌ ERROR: No GCP project configured. Set PROJECT_ID or run 'gcloud config set project <PROJECT_ID>'."
  exit 1
fi

echo "=========================================================================="
echo "🚀 Bootstrapping Wiz Demo Platform for project: ${PROJECT_ID} (${REGION})"
echo "=========================================================================="

gcloud config set project "${PROJECT_ID}" --quiet

# 1. Enable Required GCP APIs
echo "📦 1/5 Enabling GCP APIs..."
gcloud services enable \
  run.googleapis.com \
  container.googleapis.com \
  compute.googleapis.com \
  firestore.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  iap.googleapis.com \
  aiplatform.googleapis.com \
  storage.googleapis.com \
  iam.googleapis.com \
  logging.googleapis.com \
  monitoring.googleapis.com \
  --quiet

# Brief wait to ensure Google-managed Service Agents are provisioned
echo "⏳ Allowing 15s for GCP Service Agents to provision..."
sleep 15

# 2. Create Artifact Registry Repository (if not already exists)
echo "📦 2/5 Creating Artifact Registry Docker repository..."
if ! gcloud artifacts repositories describe wiz-devex-demo-images --location="${REGION}" >/dev/null 2>&1; then
  gcloud artifacts repositories create wiz-devex-demo-images \
    --repository-format=docker \
    --location="${REGION}" \
    --description="Docker repository for Wiz Demo Platform and demo images" \
    --quiet
  echo "✅ Artifact Registry repository 'wiz-devex-demo-images' created."
else
  echo "ℹ️ Artifact Registry repository 'wiz-devex-demo-images' already exists."
fi

# 3. Create & Configure Dedicated Cloud Build Service Account
echo "🔐 3/5 Setting up Dedicated Cloud Build Service Account..."
if ! gcloud iam service-accounts describe "cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" >/dev/null 2>&1; then
  gcloud iam service-accounts create cloudbuild-builder-sa \
    --display-name="Cloud Build Dedicated Builder SA" \
    --quiet
  echo "⏳ Waiting 30s for SA object creation to stabilize..."
  sleep 30
fi

# Grant IAM Roles to Builder SA
echo "🔐 Granting IAM roles to builder SA..."
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/storage.admin" \
  --quiet

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin" \
  --quiet

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer" \
  --quiet

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/logging.logWriter" \
  --quiet

# Allow Cloud Build Service Agent to act as the dedicated builder SA
PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format="value(projectNumber)")

gcloud iam service-accounts add-iam-policy-binding \
  "cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --member="serviceAccount:service-${PROJECT_NUMBER}@gcp-sa-cloudbuild.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser" \
  --quiet

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:service-${PROJECT_NUMBER}@gcp-sa-cloudbuild.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin" \
  --quiet

# Allow Compute Engine default SA to pull images from Artifact Registry
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${PROJECT_NUMBER}-compute@developer.gserviceaccount.com" \
  --role="roles/artifactregistry.reader" \
  --quiet

# Stabilization wait for IAM propagation across regional caches
echo "⏳ Allowing 45s for GCP IAM role bindings to propagate across all regions..."
sleep 45

# 4. Create Remote Terraform State Bucket
echo "🪣 4/5 Setting up Terraform State Bucket..."
export STATE_BUCKET="${PROJECT_ID}-wiz-devex-demo-tf-state"
if ! gcloud storage buckets describe "gs://${STATE_BUCKET}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${STATE_BUCKET}" --location="${REGION}" --uniform-bucket-level-access
  echo "✅ Remote state bucket 'gs://${STATE_BUCKET}' created."
else
  echo "ℹ️ Remote state bucket 'gs://${STATE_BUCKET}' already exists."
fi

# 5. Create Configurations Storage Bucket & Sync Initial Templates
echo "🪣 5/5 Setting up Configurations Storage Bucket..."
export CONFIG_BUCKET="${PROJECT_ID}-wiz-devex-demo-configs"
if ! gcloud storage buckets describe "gs://${CONFIG_BUCKET}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${CONFIG_BUCKET}" --location="${REGION}" --uniform-bucket-level-access
  echo "✅ Configurations bucket 'gs://${CONFIG_BUCKET}' created."
else
  echo "ℹ️ Configurations bucket 'gs://${CONFIG_BUCKET}' already exists."
fi

if [ -d "configurations" ]; then
  echo "📤 Syncing initial demo configurations to gs://${CONFIG_BUCKET}..."
  gcloud storage rsync configurations "gs://${CONFIG_BUCKET}" --recursive
  echo "✅ Configurations synced."
fi

echo "=========================================================================="
echo "🎉 Bootstrap complete! Next steps:"
echo "  task build          # Build frontend container image via Cloud Build"
echo "  task init           # Initialize Terraform"
echo "  task apply          # Deploy infrastructure"
echo "  task sync-configs   # Sync demo templates to storage bucket"
echo "=========================================================================="
