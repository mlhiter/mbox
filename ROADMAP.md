# ROADMAP

## Phase 0: Product and Technical Validation

Goal: confirm the product boundary for an independent Kubernetes execution platform built on `agent-sandbox` for interactive runtimes.

Deliverables:

- Product documents in this repo.
- Runtime integration plan for `agent-sandbox`.
- Kubernetes Job usage boundary for isolated batch tasks that do not need an interactive sandbox.
- Long-term technical surface split: server, web app, CLI, API docs, and SDK package.
- Minimum supported Kubernetes version decision.
- Initial namespace, RBAC, storage, and network policy model.
- Clickable console prototype or low-fidelity UI map.

Exit criteria:

- Clear product boundary around environment templates, sandboxes, runtime sessions, execution tasks, previews, artifacts, and policy.
- Confirmed first runtime adapter boundary around `agent-sandbox`.
- MVP scope small enough to implement without building an agent platform or a full CI/CD platform.

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
- Done: PVC-backed workspace projection, runtime storage metadata, and smoke coverage for persistence across Pod replacement.
- Done: simplified sandbox launch UX with generated slugs, namespace defaults, and default sandbox ServiceAccount.
- Done: Node.js workspace template defaults for local terminal and preview testing.
- Done: pending runtime workspace state with polling before terminal/logs/events/preview runtime calls.
- Done: manual preview port declaration from the Preview tab.
- Done: sandbox stop/start lifecycle actions that pause and resume the projected runtime while preserving the product record.
- Done: E2B-style template library surface that treats templates as ready-to-run environments, with Essentials first and raw image/command/policy fields behind Advanced settings.

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
- Agent planning or autonomous coding workflow.
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

## Phase 3: Runtime Sessions and Execution Tasks

Goal: make mbox useful as a programmable execution substrate for external agents, developer tools, CI systems, and human operators without making mbox own those upper-layer workflows.

Scope:

- Runtime session model for terminal, IDE, notebook, browser, command, and custom clients.
- Execution task model for controlled command or workload execution.
- Task status, logs, exit code, cancellation, timeout, and cleanup state.
- Artifact records for files, reports, screenshots, logs, build outputs, images, or links.
- Preview records beyond raw sandbox port declarations when useful.
- API and UI for inspecting sessions, tasks, logs, previews, and artifacts.
- Kubernetes Job adapter for isolated batch tasks when a full interactive sandbox is unnecessary.

Execution model:

- Interactive sandboxes remain stateful.
- Sessions attach to sandboxes.
- Tasks can run inside a sandbox when they need shared workspace state.
- Tasks can run as Kubernetes Jobs when they are isolated and repeatable.
- External agents, CI systems, IDEs, and release tools decide why work is running; mbox records and controls where and how it runs.

Exit criteria:

- A client can start a task against a sandbox and watch its status, logs, exit result, and artifacts through the API.
- A user can inspect active sessions and task history from the console.
- A canceled or timed-out task cleans up its runtime resources.
- Preview links and artifacts are attached to the runtime or task that produced them.

## Phase 3.5: CLI and SDK Foundation

Goal: make mbox usable outside the web console by developers, external agents, automation clients, and CI scripts.

Scope:

- CLI authentication and context selection.
- CLI commands for project, template, sandbox, session, task, logs, ports, previews, and artifacts.
- API schema publication for implemented resources.
- API documentation site or generated docs for public endpoints.
- First official SDK package, either Node.js or Go.
- Versioning rules across server API, CLI, docs, and SDK.

Exit criteria:

- A user can create, inspect, enter, and delete a sandbox from the CLI.
- An external client can start and watch a task through the CLI or SDK.
- API docs match the implemented server behavior.
- The SDK can authenticate and call the core sandbox, session, task, preview, and artifact APIs.

## Phase 4: Upper-layer Workflow Integrations

Goal: prove that the execution platform can support CI, preview deployment, and release automation without making those workflows the base product model.

Scope:

- Optional pipeline definition and pipeline run integration.
- Optional deployment target and deployment run integration.
- Target-scoped credential references.
- Preview namespace or staging namespace execution patterns.
- Build output and image digest artifacts.
- Rollout status and service endpoint artifacts when a deployment workflow is used.
- Approval gate integration for sensitive upper-layer workflows.

Exit criteria:

- A CI system can use mbox to run a test/build flow and collect logs plus artifacts.
- A release tool can use mbox to run a preview deployment workflow with target-scoped permissions.
- Upper-layer workflow records reference sessions, tasks, previews, and artifacts rather than bypassing them.
- Production-like targets require explicit permission and audit.

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
- YAML import/export for templates and upper-layer workflow integrations.
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
7. Done: preview port entry for declared TCP ports through the API server, including manual port add/remove from the Preview tab.
8. Done: PVC behavior is covered by runtime metadata and smoke verification.
9. Done: launch flow hides machine fields from normal users and relies on generated/defaulted slug, namespace, and ServiceAccount values.
10. Done: stop/start distinguishes pausing runtime compute from deleting the sandbox.
11. Done: template metadata for runtime type, use case, resource preset, and validation status now persists with the product record.
12. Next: richer namespace-scoped RBAC/policy handling, then runtime sessions and execution task records.

This slice proves the core runtime loop before upper-layer CI or deployment integrations expand the surface area.
