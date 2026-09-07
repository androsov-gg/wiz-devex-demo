# wiz-devex-demo

A turnkey platform for Wiz Customer Engineers (CEs) to demonstrate agentic coding tool integration in Google Cloud Argolis (`altostrat.com`) environments.

It deploys an Identity-Aware Proxy (IAP) secured **code-server** (VS Code in the browser) on **Google Cloud Run** with pre-installed Wiz CLI, Claude Code CLI, Wiz VS Code extension, and pre-packaged demo scenarios.

---

## 📋 Before You Begin (One-Time Setup)

Ensure your credentials and project permissions are ready before deploying:

### 1. Google Cloud Authentication
Authenticate `gcloud` and configure Application Default Credentials (ADC) for Terraform in a single step:
```bash
gcloud auth login --update-adc
```

### 2. Agent Platform Model Garden (Claude Sonnet 4.6)
In your Argolis project, enable **Claude Sonnet 4.6** in **Agent Platform Model Garden** (formerly Vertex AI Model Garden):
1. Open [Agent Platform Model Garden](https://console.cloud.google.com/vertex-ai/model-garden).
2. Search for **Claude Sonnet 4.6** (Anthropic).
3. Click **Enable / Accept Agreement** with these standard Argolis values:
   - **Business name**: `Google LLC`
   - **Business website**: `https://www.google.com`
   - **Contact email address**: `yourname@google.com`
   - **Business HQ location**: `United States of America`
   - **Industry**: `Telecommunications`
   - **Intended users**: `Internal employees`
   - **Intended use cases**: `"Customer demonstrations"`

---

## ⚡ Quickstart Deployment

Deploy the complete demo environment with a single interactive command:

```bash
make deploy
```
*(Alternatively: `cd deployer-cli && go run . deploy`)*

### What the Deployer does:
1. **Pre-flight Health Checks**: Verifies `gcloud`, `terraform` (>= 1.3.0), `git`, active `gcloud` auth, and Application Default Credentials (`ADC`).
2. **Interactive Configuration**: Prompts for your target GCP project ID (auto-detecting your active `gcloud` project if available) and suggests smart defaults for region and IAP access.
   - *Repeat deployments*: Automatically detects existing configuration and offers instant 1-click reuse!
3. **Cloud Prerequisites**: Enables required Google Cloud APIs idempotently, prepares Artifact Registry, and configures the Cloud Build builder service account.
4. **Container Build**: Builds and pushes the scenario container image via Cloud Build (skips rebuild if configuration hasn't changed).
5. **Terraform Provisioning**: Sets up remote state in GCS and provisions the IAP-secured Cloud Run service.
6. **Live URL**: Displays the secure Cloud Run access URL upon completion.

---

## 🛠️ Additional CLI Commands

The repository provides handy shortcuts via `make`:

```bash
# Check prerequisites and validate project health anytime
# (Prompts for Project ID or uses active gcloud / existing config)
make doctor

# Inspect deployed Cloud Run service status and live URLs
make status

# Rebuild and push container images without re-running Terraform
make build-image

# Interactively teardown a deployed scenario
make destroy
```

---

## ⚙️ Service Specifications & Cost Optimization

* **Base Image**: `codercom/code-server:latest`
* **Custom Image**: `${region}-docker.pkg.dev/${project_id}/wiz-devex-demo-images/${scenario_name}:latest` (built via Cloud Build)
* **Compute Resources**: 4 vCPU, 8 GB memory
* **CPU Allocation**: Always allocated (`cpu_idle = false`) during working hours to maintain active WebSocket connections, integrated terminals, and background language servers.
* **Access Control**: Google Identity-Aware Proxy (IAP) direct integration (`domain:username.altostrat.com`).

### ⏰ Automated Scaling & Cost Optimization (Cloud Scheduler)
To prevent unnecessary compute costs outside of business hours, automated Cloud Scheduler jobs manage service instances:
- **Scale Down**: Scales `min_instance_count` to `0` at the end of the working day (`19:00` / 7 PM Mon-Fri by default).
- **Scale Up**: Scales `min_instance_count` to `1` at the start of the working day (`08:00` / 8 AM Mon-Fri by default).
- **Configurable**: Configured interactively during deployment, or customized via `schedule_timezone` and schedule variables in `terraform.auto.tfvars`.

---

## 📂 Multi-Scenario Architecture

Demo scenarios are modular and self-contained under `configurations/`:

```
configurations/
├── wiz-code-golden-demo/          # Golden demo scenario
│   ├── Dockerfile
│   ├── cloudbuild.yaml
│   ├── entrypoint.sh
│   ├── claude-code/
│   │   ├── CLAUDE.md
│   │   └── settings.json
│   └── terraform/                 # Scenario-specific Terraform configuration
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
│
└── <future-scenario>/             # Drop new scenario folders here
    ├── Dockerfile
    ├── cloudbuild.yaml
    └── terraform/
```

- Each scenario has its own independent Terraform definition and container configuration.
- Scenarios share the same remote state bucket (`${PROJECT_ID}-wiz-devex-demo-tf-state`). For backward compatibility with existing deployments, `wiz-code-golden-demo` uses the root state path, while subsequent scenarios specify `prefix = "scenarios/<scenario-name>"`.
