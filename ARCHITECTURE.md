# ARCHITECTURE

## Architecture Goal

mbox should provide a product-level control plane for Kubernetes-backed sandboxes, pipelines, and deployments while keeping the product contract decoupled from runtime CRDs.

The selected interactive sandbox runtime is `kubernetes-sigs/agent-sandbox`. mbox still owns the product API, permissions, UI, and pipeline orchestration.

Current implementation status:

- Go server entrypoint: `cmd/mbox-server`.
- Web console: separate Vite React app under `web/`.
- Product state: Postgres through `pgx`.
- Implemented product resources: projects, environment templates, and sandboxes.
- Implemented API surface: `GET /healthz` plus CRUD routes under `/v1/projects`, `/v1/templates`, and `/v1/sandboxes`.
- Runtime projection: opt-in `agent-sandbox` adapter and sandbox reconciler.

See `docs/server-api.md` for the current concrete API and configuration contract, and `docs/web-console.md` for the current frontend structure.

## High-level Layers

```text
Client Surfaces
  Web console, CLI, API docs, SDK packages

Control Plane
  Go API server, Auth, RBAC, state, scheduling decisions, audit, lifecycle management

Pipeline Orchestrator
  Step state machine, logs, retries, cancellation, artifacts, deployments

Runtime Adapter
  agent-sandbox for interactive sandboxes, Kubernetes Job where CI steps need isolated execution

Kubernetes
  agent-sandbox CRDs, Namespaces, Pods, PVCs, Services, Ingress/Gateway, NetworkPolicy, RBAC
```

## Long-term Technical Surfaces

mbox should grow into several coordinated technical surfaces rather than a single web application.

### Server

The server side includes:

- Go API server for product APIs.
- Controllers/reconcilers for product records and Kubernetes runtime resources.
- `agent-sandbox` integration for interactive sandbox runtime.
- Kubernetes resources, RBAC, NetworkPolicy, PVCs, Services, Gateway or Ingress, logs, and events.

The server is the source of truth for projects, templates, sandboxes, pipelines, deployment targets, policies, credentials, audit, and runtime state mapping.

### Web App

The web app is the human-facing console for daily operation:

- project, sandbox, template, pipeline, deployment, policy, and credential management
- terminal, IDE, notebook, preview port, logs, events, and status views
- dense operational workflows for repeated use

The web app should consume the same product APIs as the CLI and SDK. It should not depend on private controller behavior or raw Kubernetes objects as its main contract.

The current web app is a Vite React app in `web/`. In local development it runs separately from the Go API server and proxies `/healthz` plus `/v1/*` to the configured API target. The default local split is API on `127.0.0.1:18080` and Vite on `127.0.0.1:5174`.

### CLI

The CLI should provide scriptable access to the same core workflows:

- project and template inspection
- sandbox create, enter, list, stop, delete
- port and log access
- pipeline run, watch, cancel, retry
- deployment status and rollback

The CLI should be suitable for local developer use, CI scripts, and operational debugging. It should be a first-class API client, not a separate control path.

### API Docs

The API documentation surface should publish the product API contract for humans and automation clients:

- authentication model
- resource schemas
- request and response examples
- error codes and policy denial reasons
- streaming/log/watch semantics
- versioning and compatibility rules

The API docs should track the server API version and SDK generation boundary.

### SDK Package

mbox should provide at least one official SDK/package for automation clients. The first package can be Node.js or Go, depending on the first integration audience.

The SDK should wrap the public product API for common workflows while keeping raw API access possible for advanced clients. It should share API schemas with the server and API docs where practical.

## Runtime Boundary

mbox uses `agent-sandbox` as the interactive sandbox runtime. It should not treat `agent-sandbox` as the whole product.

The product should depend on a stable internal runtime contract:

```text
CreateRuntime
StartRuntime
StopRuntime
PauseRuntime
ResumeRuntime
RunCommand
StreamLogs
ExposePort
AttachVolume
DeleteRuntime
GetRuntimeStatus
```

For interactive sandboxes, the adapter creates `SandboxClaim` resources and maps them to mbox `Sandbox` records.

For CI/CD jobs, mbox can use sandbox-backed execution when the run needs an interactive or stateful workspace. Short, isolated, repeatable steps can use ordinary Kubernetes Jobs.

## Core Components

### API Server

Owns product APIs:

- projects
- templates
- sandboxes
- pipeline definitions
- pipeline runs
- deployment targets
- policies
- credentials and secret references
- audit records

The API server should persist desired state and user intent. It should not rely on the runtime Pod as the only source of truth.

### Web Console

Human-facing console for:

- creating and entering sandboxes
- editing environment templates
- managing pipelines
- observing deployment state
- managing resource and security policies

The UI should be operational and dense enough for repeated use. Avoid landing-page style composition in the app surface.

Current implemented console scope is intentionally narrower: list and create projects, templates, and sandboxes; inspect selected resource IDs and runtime state; show API health and request errors. Terminal access, logs, events, pipelines, deployments, and policy screens are still roadmap work.

### Controller / Reconciler

Reconciles mbox product records to Kubernetes resources:

- project namespace
- service account and RBAC
- runtime resources
- PVCs
- Services
- Gateway or Ingress routes
- NetworkPolicy
- cleanup jobs

The reconciler must be idempotent and safe under retries.

### Pipeline Orchestrator

Owns pipeline run state:

- queued
- running
- succeeded
- failed
- canceled
- waiting for approval
- rolling back

Each pipeline step should have explicit status, timing, logs, retry count, runtime reference, and failure reason.

### Runtime Adapter

The runtime adapter translates product intent into concrete execution resources.

Selected runtime adapters:

- `agent-sandbox` for interactive, stateful, singleton sandbox environments.
- Kubernetes Job for isolated, repeatable CI steps when a full sandbox is unnecessary.
- Future custom runner for specialized build or deployment execution if required.

## Kubernetes Model

### Namespace Strategy

Default to namespace-scoped isolation.

Recommended starting model:

- One project namespace for long-lived project resources.
- Optional per-run or per-sandbox namespace when stronger isolation is needed.
- Separate system namespace for mbox controllers and API services.

### RBAC Strategy

Use scoped service accounts:

- user sandbox service account
- pipeline execution service account
- deployment service account per target
- controller service account

Do not mount broad kubeconfigs into user sandboxes. Deployment permissions should be target-scoped.

### Storage Strategy

Support both:

- persistent workspace volume for interactive sandboxes
- ephemeral volumes for CI steps

Long-lived volumes need cleanup rules, quota visibility, and ownership metadata.

### Network Strategy

NetworkPolicy should default to restricted ingress and controlled egress.

Common policies:

- allow web console or gateway to reach sandbox exposed ports
- allow package registry and Git endpoints
- optionally block cluster-internal network access
- optionally allow project namespace services

### Credential Strategy

Credentials should be injected narrowly:

- Git credentials are repo-scoped.
- Registry credentials are project- or pipeline-scoped.
- Kubernetes deployment credentials are target-scoped.
- Production credentials require explicit permission and audit.

Prefer short-lived tokens and secret references over copying long-lived credentials into runtime filesystems.

## Data Model Draft

The implemented data model currently covers the first sandbox control-plane slice. Pipeline, deployment, policy, credential, audit, and observability records below are still roadmap items.

### Project

- id
- name
- slug
- repository URL
- default namespace
- default template id
- metadata
- created at
- updated at

### EnvironmentTemplate

- id
- project id, optional for global templates
- name
- slug
- image
- startup command
- working directory
- CPU, memory, and storage requests
- exposed ports
- env
- secret references
- network policy
- lifecycle policy
- created at
- updated at

### Sandbox

- id
- project id
- template id
- name
- slug
- status
- namespace
- service account name
- runtime reference
- ports
- metadata
- created at
- updated at
- deleted at

Implemented Postgres constraints:

- UUID primary keys are generated by Postgres.
- Active sandbox slugs are unique per project while deleted sandbox slugs can be reused.
- Global template slugs are unique globally.
- Project template slugs are unique per project.
- Sandbox deletion is soft deletion through `deleted_at`.
- `updated_at` is maintained by database triggers.

### PipelineDefinition

- id
- project id
- name
- trigger mode
- steps
- default runtime policy
- allowed targets

### PipelineRun

- id
- pipeline definition id
- project id
- status
- current step
- runtime references
- logs
- artifacts
- started at
- finished at

### DeploymentTarget

- id
- project id
- name
- namespace or cluster reference
- service account reference
- approval policy
- allowed users and groups

### Policy

- id
- scope
- resource limits
- allowed images
- allowed registries
- allowed network destinations
- lifecycle rules
- credential access rules

## Security Requirements

- Default namespace isolation.
- No cluster-admin credentials in ordinary runtime environments.
- Separate human, pipeline, deployment, and controller permissions.
- Explicit audit for secret use and deployment actions.
- NetworkPolicy enabled for sandbox namespaces.
- RuntimeClass support for stronger isolation when available.
- Image provenance and digest display for deployments.
- Cleanup controller for stale sandboxes and volumes.

## Observability Requirements

The product should expose:

- sandbox phase and pod status
- container logs
- pipeline step logs
- Kubernetes events
- resource usage
- deployment rollout status
- failed scheduling reasons
- image pull errors
- quota and policy denial reasons

## Runtime Implementation Notes

`agent-sandbox` is the selected interactive sandbox runtime because it provides Kubernetes-native CRDs for stateful singleton workloads and claims/templates/warm pools.

Do not couple product APIs to `SandboxClaim` directly. Store mbox product records and map them to runtime resources through an adapter. This keeps the mbox product model stable and prevents the UI/API from becoming a thin wrapper around runtime CRDs.

The implemented runtime controller is intentionally opt-in through `MBOX_RUNTIME_CONTROLLER_ENABLED=true`. With the controller disabled, the server only writes mbox product records in Postgres. With it enabled, the reconciler projects eligible sandbox records into Kubernetes by creating or updating a namespace, scoped ServiceAccount, `SandboxTemplate`, and `SandboxClaim`.

The generated sandbox ServiceAccount and pod template both set token automount to false. Runtime credentials should be introduced later as narrow, explicit capabilities rather than inherited cluster access.
