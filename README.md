# wiz-devex-demo

## 🎯 Project Goal
The goal of this project is to provide a stable and extendable platform for Wiz CEs to demonstrate how the product can be integrated into a developer's environment, specifically focusing on the use of agentic coding tools.

This repository provides an automated deployment of **code-server** running on **Google Cloud Run** secured by **Identity-Aware Proxy (IAP)**, designed specifically for Google Cloud Argolis (`altostrat.com`) environments.

It deploys an IAP-secured Cloud Run service running a custom **code-server** container image (pre-installing Wiz CLI, Claude Code CLI, Wiz VS Code extension, and pre-packaged configurations) built prior to running Terraform without requiring a VPC network.

### Built-in Service Specifications
* **Base Image**: `codercom/code-server:latest`
* **Custom Image**: `${region}-docker.pkg.dev/${project_id}/wiz-devex-demo-images/${scenario_name}:latest` (built via Cloud Build)
* **Compute Resources**: 4 vCPU, 8 GB memory
* **CPU Allocation**: Always allocated (`cpu_idle = false`) during working hours to maintain active WebSocket connections, integrated terminals, and background language servers.
* **Instance Scaling & Scheduling**: Automated Cloud Scheduler jobs scale min instances down to 0 at the end of the working day (e.g. 7 PM Mon-Fri) and back up to 1 at the start of the day (e.g. 8 AM Mon-Fri) in a configurable timezone.
* **Access Control**: Google Identity-Aware Proxy (IAP) direct integration

---

### ⏰ Automated Scaling & Cost Optimization (Cloud Scheduler)

To prevent unnecessary Cloud Run compute costs outside of business hours, the infrastructure includes automated Google Cloud Scheduler jobs:
- **Scale Down**: Scales `min_instance_count` to `0` at the end of the working day (`scale_down_schedule = "0 19 * * 1-5"` by default).
- **Scale Up**: Scales `min_instance_count` to `1` at the start of the working day (`scale_up_schedule = "0 8 * * 1-5"` by default).
- **Configurable**: Configure your local timezone (`schedule_timezone`), custom cron hours, or disable via `enable_scheduler = false` in `terraform.auto.tfvars`.

## ⚡ Quickstart Deployment (Recommended)

Once you clone this repository, you can deploy the complete demo environment with a single interactive command:

```bash
make deploy
```
*(Alternatively, run `cd deployer-cli && go run . deploy` or `./bin/deployer deploy`)*

### What the Deployer CLI does automatically:
1. **Pre-flight Health Checks**: Verifies `gcloud`, `terraform`, `git`, and active GCP authentication.
2. **Scenario Selection**: Discovers demo scenarios in `configurations/` and lets you select which scenario to deploy.
3. **Auto-Configuration**: Discovers active GCP project/account and prompts for confirmation with smart defaults.
4. **Cloud Prerequisites**: Automatically enables required APIs, sets up Artifact Registry, configures Cloud Build builder Service Account & IAM permissions.
5. **Container Build**: Submits the container build to Cloud Build and pushes to Artifact Registry.
6. **Terraform Provisioning**: Sets up remote state and interactively plans and applies infrastructure.
7. **Demo Guide**: Prints the live IAP Cloud Run URL and demo instructions.

---

## 🛠️ Additional CLI Commands

The repository provides handy shortcuts via `make`:

```bash
# Check prerequisites and Model Garden checklist
make doctor

# Inspect deployed Cloud Run service status and live URLs
make status

# Rebuild and push container images without re-running Terraform
make build-image

# Interactively teardown a deployed scenario
make destroy
```

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

---

## 📋 One-Time Prerequisite: Claude Sonnet 4.6 on Vertex AI Model Garden

In the Argolis project where you are deploying this demo, enable **Claude Sonnet 4.6** in **Vertex AI Model Garden**:
1. Open [Vertex AI Model Garden](https://console.cloud.google.com/vertex-ai/model-garden).
2. Search for **Claude Sonnet 4.6** (Anthropic).
3. Click **Enable / Accept Agreement** with these standard Argolis values:
   - **Business name**: `Google LLC`
   - **Business website**: `https://www.google.com`
   - **Contact email address**: `yourname@google.com`
   - **Business HQ location**: `United States of America`
   - **Industry**: `Telecommunications`
   - **Intended users**: `Internal employees`
   - **Intended use cases**: `"Customer demonstrations"`
