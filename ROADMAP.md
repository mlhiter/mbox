# ROADMAP

## Phase 0: Product and Technical Validation

Goal: confirm the product boundary for an independent Kubernetes sandbox and CI/CD control plane built on `agent-sandbox` for interactive runtimes.

Deliverables:

- Product documents in this repo.
- Runtime integration plan for `agent-sandbox`.
- Kubernetes Job usage boundary for CI steps that do not need an interactive sandbox.
- Long-term technical surface split: server, web app, CLI, API docs, and SDK package.
- Minimum supported Kubernetes version decision.
- Initial namespace, RBAC, storage, and network policy model.
- Clickable console prototype or low-fidelity UI map.

Exit criteria:

- Clear product boundary around environment, sandbox, pipeline, deployment, and policy.
- Confirmed first runtime adapter boundary around `agent-sandbox`.
- MVP scope small enough to implement without building a full CI platform at once.

## Phase 1: Core Sandbox MVP

Goal: let a human create, open, inspect, and destroy a Kubernetes-backed sandbox.

Current progress:

- Done: initial Go API server.
- Done: Postgres-backed Project, EnvironmentTemplate, and Sandbox models.
- Done: CRUD routes for `/v1/projects`, `/v1/templates`, and `/v1/sandboxes`.
- Done: opt-in sandbox reconciler that projects Sandbox records to `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.
- Done: basic runtime status mapping from `SandboxClaim` Ready condition to mbox sandbox status.
- Done: separate Vite web console with project, template, and sandbox list/create/inspect workflows.
- Done: Notion-adjacent console design system captured in `DESIGN.md`.
- Done: browser terminal for running sandboxes through Kubernetes `pods/exec`.
- Done: runtime target, logs, and Kubernetes events API routes plus main-workspace runtime tabs.
- Done: real `agent-sandbox` cluster smoke verification against `kind-agent-sandbox`.
- Done: declared preview port metadata and API-proxied open links for running sandbox TCP ports.

Scope:

- Project model.
- Environment template model.
- Sandbox model.
- Runtime adapter for `agent-sandbox`.
- Initial Go API server.
- Initial controller/reconciler.
- Basic web console.
- Terminal access.
- Basic port listing and preview endpoint.
- PVC-backed workspace.
- Sandbox lifecycle: create, start, stop, delete.
- Basic logs and events view.
- Minimal API documentation for the implemented sandbox endpoints.

Out of scope:

- Full pipeline editor.
- Production deployment target.
- Full SDK package.
- Multi-cluster support.
- Billing.

Exit criteria:

- A user can create a sandbox from a template.
- A user can enter the sandbox from the browser.
- A user can expose and open a preview port.
- A user can see runtime status, logs, and Kubernetes events.
- A user can delete the sandbox and associated resources.

## Phase 2: Template and Policy Management

Goal: make sandboxes reusable and governable.

Scope:

- Template creation and editing UI.
- Template validation by launching a test sandbox.
- Resource presets for CPU, memory, GPU, and storage.
- Environment variables and secret references.
- Lifecycle policy: TTL, idle timeout, auto cleanup.
- Namespace-scoped RBAC setup.
- Network policy presets.
- Quota display.

Exit criteria:

- Platform users can create a template and make it available to a project.
- Ordinary users can launch only allowed templates.
- Sandboxes respect resource and lifecycle policies.
- Secret references are visible as references, not leaked values.

## Phase 3: Pipeline MVP

Goal: run repeatable CI-style workflows in controlled Kubernetes execution environments.

Scope:

- Pipeline definition model.
- Pipeline run model.
- Step runner.
- Step logs.
- Retry and cancel.
- Git checkout step.
- Test step.
- Build image step.
- Push image step.
- Artifact and image digest recording.
- UI for run status and step logs.

Execution model:

- Interactive sandboxes remain stateful.
- Pipeline steps can use Kubernetes Jobs by default.
- Long-running or debugging-oriented pipeline runs can optionally attach to a sandbox.

Exit criteria:

- A user can configure and run a simple test/build/push pipeline.
- A failed step shows readable failure reason and logs.
- A canceled run stops its runtime resources.
- Image output is recorded by digest.

## Phase 3.5: CLI and SDK Foundation

Goal: make mbox usable outside the web console by developers, automation clients, and CI scripts.

Scope:

- CLI authentication and context selection.
- CLI commands for project, template, sandbox, logs, ports, and pipeline runs.
- API schema publication for implemented resources.
- API documentation site or generated docs for public endpoints.
- First official SDK package, either Node.js or Go.
- Versioning rules across server API, CLI, docs, and SDK.

Exit criteria:

- A user can create, inspect, enter, and delete a sandbox from the CLI.
- A CI script can trigger and watch a pipeline run through the CLI or SDK.
- API docs match the implemented server behavior.
- The SDK can authenticate and call the core sandbox and pipeline APIs.

## Phase 4: Deployment Targets

Goal: deploy pipeline output to controlled preview or staging environments.

Scope:

- Deployment target model.
- Target-scoped service accounts.
- Preview namespace deployment.
- Rollout status.
- Service endpoint display.
- Rollback.
- Deployment logs and events.
- Approval gate for sensitive targets.

Exit criteria:

- A user can deploy a built image to a preview target.
- The UI shows rollout status and access URL.
- A user can rollback to a prior revision.
- Production-like targets require explicit permission.

## Phase 5: Operational Hardening

Goal: make the platform safe enough for shared clusters.

Scope:

- Audit logs.
- Admission checks for allowed images and resources.
- Stronger runtime isolation with RuntimeClass where available.
- Network egress controls.
- Idle detection.
- Orphan resource cleanup.
- Quota enforcement.
- Multi-tenant metrics.
- Backup and restore plan for control-plane state.

Exit criteria:

- Operators can identify who created, accessed, deployed, or deleted each resource.
- Stale sandboxes and volumes are cleaned safely.
- Policy denial reasons are visible in the UI.
- Runtime resource usage is observable at project and user levels.

## Phase 6: Advanced Platform Capabilities

Goal: expand from MVP to mature platform.

Candidate features:

- Warm pools for faster sandbox startup.
- Browser and notebook sandbox templates.
- GPU templates.
- Multi-cluster deployment targets.
- YAML import/export for templates and pipelines.
- Git provider integrations.
- Registry browsing and image promotion.
- Fine-grained approval workflows.
- Scheduled pipelines.
- API tokens for automation clients.
- Runtime adapter plugin system.

These should be pulled into earlier phases only if they unblock the MVP, not because they are generally useful.

## Near-term First Slice

First slice status:

1. Done: API server with Projects, Templates, and Sandboxes.
2. Done: controller that creates one `agent-sandbox` runtime per Sandbox record when explicitly enabled.
3. Done: Vite web console with project list, template list, sandbox list, create dialogs, resource inspection, and main-area Runtime Workspace.
4. Done: default BusyBox smoke template proving terminal-ready sandbox startup.
5. Done: terminal access plus logs/events in the web console runtime workspace.
6. Done: real cluster smoke verification for create, runtime access, exec, status mapping, and cleanup.
7. Done: basic preview port entry for declared TCP ports through the API server.
8. Next: PVC behavior and richer namespace-scoped RBAC/policy handling.

This slice proves the core product loop before CI/CD expands the surface area.
