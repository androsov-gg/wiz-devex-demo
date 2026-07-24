# wiz-devex-demo - 🚧 Work in Progress 🚧

## 🎯 Project Goal
The goal of this project is to provide a stable and extendable platform for Wiz CEs to demonstrate how the product can be integrated into a developer's environment, specifically focusing on use of agentic coding tools.

This repository provides a deployment of **code-server** running on **Google Cloud Run** secured by **Identity-Aware Proxy (IAP)**. This is designed specifically for Google Cloud Argolis (`altostrat.com`) environments.

It deploys an IAP-secured Cloud Run service running a custom **code-server** container image (pre-installing Wiz CLI, Claude Code CLI, Wiz VS Code extension, and pre-packaged configurations) built prior to running Terraform without requiring a VPC network.

### Built-in Service Specifications
* **Base Image**: `codercom/code-server:latest`
* **Custom Image**: `${region}-docker.pkg.dev/${project_id}/wiz-devex-demo-images/wiz-code-golden-demo:latest` (built via Cloud Build)
* **Compute Resources**: 4 vCPU, 8 GB memory
* **CPU Allocation**: Always allocated (`cpu_idle = false`) to maintain active WebSocket connections, integrated terminals, and background language servers.
* **Instance Scaling**: 1 min instance, 1 max instance (always-on 1 service running)
* **Access Control**: Google Identity-Aware Proxy (IAP) direct integration

---

## 🚀 Deployment Instructions

### 1. Prerequisites: Environment, Artifact Registry, SA & Image Build
Before provisioning infrastructure with Terraform, configure your GCP project environment, enable required APIs, grant Model Garden access, create the Artifact Registry repository, configure the Cloud Build Service Account, and submit the container build.

> [!IMPORTANT]
> The only supported regions are europe-west1, us-east5 or asia-southeast1.

1. **Setup project / region environment variables & configure `gcloud`**:
   ```bash
   export PROJECT_ID="your-argolis-project-id"
   export REGION="your-region1"

   gcloud config set project "${PROJECT_ID}"
   ```

2. **Enable required GCP APIs**:
   ```bash
   gcloud services enable \
     run.googleapis.com \
     aiplatform.googleapis.com \
     storage.googleapis.com \
     iam.googleapis.com \
     cloudbuild.googleapis.com \
     artifactregistry.googleapis.com \
     iap.googleapis.com
   ```

3. **Enable Claude Sonnet 4.6 in Agent Platform Model Garden**:
   In the Argolis project that you are going to deploy this demo in, enable **Claude Sonnet 4.6** in **Agent Platform Model Garden**. Supply the following details:
   - **Business name**: `Google LLC`
   - **Business website**: `https://www.google.com`
   - **Contact email address**: `yourname@google.com`
   - **Business HQ location**: `United States of America`
   - **Industry**: `Telecommunications`
   - **Intended users**: `Internal employees`
   - **Intended use cases**: `"Customer demonstrations"`

4. **Create Artifact Registry Repository**:
   ```bash
   gcloud artifacts repositories create wiz-devex-demo-images \
     --repository-format=docker \
     --location="${REGION}" \
     --description="Docker repository for code-server Cloud Run images"
   ```

5. **Set Up Service Account for Cloud Build**:
   ```bash
   # Create Dedicated Builder Service Account
   gcloud iam service-accounts create cloudbuild-builder-sa \
     --display-name="Cloud Build Dedicated Builder SA"
   ```

   Wait 60 seconds before running the policy bindings, to allow the SA to stabilise.
   ```bash
   # Grant required IAM roles to the Dedicated Builder Service Account
   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role="roles/storage.admin"

   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role="roles/storage.objectAdmin"

   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role="roles/artifactregistry.writer"

   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role="roles/logging.logWriter"

   # Allow Cloud Build Service Agent to act as the dedicated builder SA
   PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format="value(projectNumber)")

   gcloud iam service-accounts add-iam-policy-binding \
     cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com \
     --member="serviceAccount:service-${PROJECT_NUMBER}@gcp-sa-cloudbuild.iam.gserviceaccount.com" \
     --role="roles/iam.serviceAccountUser"

   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:service-${PROJECT_NUMBER}@gcp-sa-cloudbuild.iam.gserviceaccount.com" \
     --role="roles/storage.objectAdmin"
   ```

6. **Build & Push Container Image**:
   ```bash
   export IMAGE_TAG="${REGION}-docker.pkg.dev/${PROJECT_ID}/wiz-devex-demo-images/wiz-code-golden-demo:latest"

   gcloud builds submit \
     --project="${PROJECT_ID}" \
     --region="${REGION}" \
     --service-account="projects/${PROJECT_ID}/serviceAccounts/cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --config="configurations/wiz-code-golden-demo/cloudbuild.yaml" \
     --substitutions=_IMAGE_TAG="${IMAGE_TAG}" \
     configurations/wiz-code-golden-demo
   ```

### 2. Configure Variables & Initialize Remote State
Copy `terraform.tfvars.example` to `terraform.auto.tfvars`, set your GCP `project_id` and `region`, create the state bucket (`${PROJECT_ID}-wiz-devex-demo-tf-state`), and initialize Terraform:

```bash
cp terraform.tfvars.example terraform.auto.tfvars
# Edit project_id, region, and the allowed domains in terraform.auto.tfvars

export STATE_BUCKET="${PROJECT_ID}-wiz-devex-demo-tf-state"

gcloud storage buckets create "gs://${STATE_BUCKET}" --location="${REGION}" --uniform-bucket-level-access

terraform init -backend-config="bucket=${STATE_BUCKET}"
```

### 3. Apply Terraform Configuration
Deploy the Cloud Run service secured with IAP using the pre-built container image:

```bash
terraform apply
```

### 4. Accessing code-server
Once Terraform completes:
1. Copy the outputted `cloud_run_service_url` (`https://wiz-code-golden-demo-xxxxx-xx.a.run.app`).
2. Open the URL in your browser. Google IAP will authenticate your Argolis account before granting access to code-server with all pre-loaded Wiz tools and repositories ready!

---

## 🧹 Teardown Instructions

To clean up Cloud Run services and IAM bindings provisioned by Terraform:

```bash
terraform destroy
```

Note: Pre-requisite resources created outside Terraform (such as the Artifact Registry repository and Cloud Build Service Account) can be deleted separately if desired using:
```bash
gcloud artifacts repositories delete wiz-devex-demo-images --location="${REGION}" --quiet
gcloud iam service-accounts delete "cloudbuild-builder-sa@${PROJECT_ID}.iam.gserviceaccount.com" --quiet
```

