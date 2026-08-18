# wiz-devex-demo (Wiz Demo Platform)

## 🎯 Overview
The **Wiz Demo Platform** is a sales demonstration and solution engineering hub running on **Google Cloud Run** (Frontend Portal & Dynamic Demo Workloads) in **Google Cloud Argolis (`altostrat.com`)** environments.

It allows sales engineers (CEs) to browse a catalog of pre-configured demo environments (such as the **Wiz Code & Claude Code Golden Demo**), launch isolated demo instances on-demand as dedicated Cloud Run v2 services, monitor their lifecycle in real-time, and access them directly through a **Google-Managed Identity-Aware Proxy (IAP)** perimeter with **Zero OAuth Credentials** required.

### 📦 Decoupled Configuration Architecture
The platform portal contains **no hardcoded demo configurations**. Instead, demo templates and versioned files are dynamically loaded from a Google Cloud Storage bucket (`gs://<PROJECT_ID>-wiz-devex-demo-configs`).

The storage bucket is populated from the `configurations/` folder in this repository:
```
configurations/
  index.yaml                         # Catalog metadata: { configName: { title, category, versions: { version, description } } }
  <configName>/                      # e.g. wiz-code-golden-demo
    <version>/                       # e.g. 1.0.0 (Semantic Versioning)
      Dockerfile                     # Container image specification
      cloudbuild.yaml                # On-demand build definition
      entrypoint.sh                  # Workload runtime script
      vscode_settings.json           # VS Code / code-server defaults
      claude-code/                   # Claude Code commands, settings & prompts
```

---

## ⚡ Quick Start (Taskfile)

All deployment, building, configuration syncing, and management tasks are automated with [Taskfile](https://taskfile.dev).

### 1. Prerequisites

1. **Authenticate Application Default Credentials (ADC) & configure project**:
   ```bash
   # Authenticate Application Default Credentials (ADC)
   gcloud auth application-default login

   # Set project and region
   export PROJECT_ID="your-argolis-project-id"
   export REGION="europe-west1" # Supported: europe-west1, us-east5, asia-southeast1, us-central1

   gcloud config set project "${PROJECT_ID}"
   ```

2. **Enable Claude Sonnet 4.6 in Agent Platform Model Garden**:
   In the Argolis project where you are deploying, enable **Claude Sonnet 4.6** in **Agent Platform Model Garden**. Supply the following details:
   - **Business name**: `Google LLC`
   - **Business website**: `https://www.google.com`
   - **Contact email address**: `yourname@google.com`
   - **Business HQ location**: `United States of America`
   - **Industry**: `Telecommunications`
   - **Intended users**: `Internal employees`
   - **Intended use cases**: `"Customer demonstrations"`

---

### 2. Bootstrap Environment
Run the 1-click bootstrap task to enable APIs, create the Artifact Registry repository, configure Cloud Build permissions, and create the state & configurations buckets:

```bash
task bootstrap
```

---

### 3. Build & Deploy

1. Customize `terraform.auto.tfvars`:
   ```bash
   cp terraform.tfvars.example terraform.auto.tfvars
   # Update project_id, region, and iap_members in terraform.auto.tfvars
   ```

2. Build, Deploy, and Sync Configurations:
   ```bash
   task init
   task deploy
   ```
   *(Or individually: `task build`, `task apply`, and `task sync-configs`).*

3. Get your Portal URL:
   ```bash
   task url
   ```
   Open the returned URL in your browser. Google IAP will authenticate your Argolis account seamlessly. Demo images are automatically built on demand via Cloud Build when launched from the UI.

---

## 🛠️ Available Tasks

| Command | Description |
| :--- | :--- |
| `task bootstrap` | 🚀 1-Click setup for Argolis APIs, Artifact Registry, Cloud Build SA, and state/config buckets |
| `task sync-configs` | 📤 Sync the `configurations/` directory to the GCS configurations bucket |
| `task build` | 🏗️ Build and push the Next.js Frontend Portal container image via Cloud Build |
| `task init` | 🔧 Initialize Terraform with the remote GCS state bucket |
| `task plan` | 📋 Run `terraform plan` |
| `task apply` | ⚡ Apply Terraform configuration (VPC, Cloud NAT, GCS, Firestore, Cloud Run IAP) |
| `task deploy` | 🚀 Complete deployment (`task build` + `task apply` + `task sync-configs`) |
| `task url` | 🌐 Display the IAP-secured Portal URL |
| `task destroy` | 🧹 Tear down all provisioned infrastructure |

---

## 🧹 Teardown

To clean up all Cloud Run, Firestore, and GCP resources:
```bash
task destroy
```
