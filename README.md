# wiz-devex-demo (:construction: Work in Progress :construction:)

## 🎯 Project Goal
The goal of this project is to provide a stable and extendable platform for Wiz CEs to demonstrate how the product can be integrated into a developer's environment, specifically focusing on use of agentic coding tools.

This repository provides a deployment of **code-server** running on **Google Cloud Run** secured by **Identity-Aware Proxy (IAP)**. This is designed specificially for for Google Cloud Argolis (`altostrat.com`) environments.

It provisions an Artifact Registry Docker repository, triggers **Cloud Build** as part of Terraform to build a custom container image (pre-installing Wiz CLI, Claude Code CLI, Wiz VS Code extension, and pre-packaged configurations), and deploys an IAP-secured Cloud Run service without requiring a VPC network.

### Built-in Service Specifications
* **Base Image**: `codercom/code-server:latest`
* **Custom Image**: `${region}-docker.pkg.dev/${project_id}/wiz-devex-demo-images/wiz-code-golden-demo:latest` (built via Cloud Build)
* **Compute Resources**: 4 vCPU, 8 GB memory
* **CPU Allocation**: Always allocated (`cpu_idle = false`) to maintain active WebSocket connections, integrated terminals, and background language servers.
* **Instance Scaling**: 1 min instance, 1 max instance (always-on 1 service running)
* **Access Control**: Google Identity-Aware Proxy (IAP) direct integration

---

## 🚀 Deployment Instructions

### 1. Prerequisites: Setting Up Access & APIs
Before provisioning infrastructure with Terraform, configure your GCP project environment, enable required APIs, and grant Model Garden access.

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
Provision Artifact Registry, trigger Cloud Build to compile the custom code-server container image, and deploy the Cloud Run service secured with IAP:

```bash
terraform apply
```

### 4. Accessing code-server
Once Terraform completes:
1. Copy the outputted `cloud_run_service_url` (`https://wiz-code-golden-demo-xxxxx-xx.a.run.app`).
2. Open the URL in your browser. Google IAP will authenticate your Argolis account before granting access to code-server with all pre-loaded Wiz tools and repositories ready!

---

## 🧹 Teardown Instructions

To clean up all Cloud Run services, Artifact Registry images, and IAM bindings when finished presenting:

```bash
terraform destroy
```
