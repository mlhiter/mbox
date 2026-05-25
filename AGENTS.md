# AGENTS.md

## Project Identity

mbox is an independent Kubernetes sandbox and CI/CD control plane for people and automation.

The product surface is human-first: environment templates, sandboxes, pipelines, deployments, policies, credentials, logs, and operational state.

Long-term technical surfaces:

- Server side: Go API server, controllers, `agent-sandbox` integration, and Kubernetes resources.
- Web app: human-facing operational console.
- CLI: scriptable operation for developers, CI, and platform users.
- API docs: published product API contract.
- SDK package: Node.js or Go package for automation clients.

## Product Boundaries

Build:

- Kubernetes-backed sandbox management.
- Human-facing web console.
- Environment template management.
- CI/CD pipeline configuration and execution.
- Deployment targets and preview/staging workflows.
- Namespace-scoped policy, RBAC, credentials, quota, lifecycle, and observability.

Keep the product centered on the mbox primitives above. New modules should directly strengthen sandbox creation, environment configuration, pipeline execution, deployment operation, policy enforcement, credential handling, or observability.

## Architecture Principles

- Use `agent-sandbox` as the selected interactive sandbox runtime adapter, not as the product API.
- Treat server API, web app, CLI, API docs, and SDK package as coordinated surfaces over the same product model.
- Keep mbox product records separate from Kubernetes runtime resources.
- Keep interactive sandboxes and CI/CD pipeline jobs related but not identical.
- Prefer namespace-scoped isolation inside a shared Kubernetes cluster.
- Use Kubernetes resources, events, logs, RBAC, and NetworkPolicy as first-class primitives.
- Keep runtime credentials narrow and scoped.
- Avoid mounting broad kubeconfigs or long-lived tokens into ordinary sandboxes.
- Make runtime adapter replacement possible.

## Development Rules

- Read the relevant documents before changing direction:
  - `PRODUCT.md`
  - `ARCHITECTURE.md`
  - `ROADMAP.md`
  - `docs/server-api.md`
  - `docs/web-console.md`
  - `docs/research-agent-sandbox.md`
- When adding implementation, keep docs current if product or architecture decisions change.
- Prefer small vertical slices over broad scaffolding.
- Preserve human-first UI language.
- Do not introduce database write operations against external databases unless the user explicitly asks.
- For container images that need publishing, build `linux/amd64` by default unless the user asks for ARM.
- For test-time cloud image pushes, default to `crpi-7jr40k6elhldekqp.cn-hangzhou.personal.cr.aliyuncs.com/mlhiter` unless instructed otherwise.

## Current Implementation Notes

- The current server entrypoint is `cmd/mbox-server`.
- `DATABASE_URL` is required; startup runs embedded Postgres migrations.
- The implemented HTTP surface is `GET /healthz`, CRUD for `/v1/projects`, `/v1/templates`, and `/v1/sandboxes`, sandbox lifecycle routes `POST /v1/sandboxes/{id}/stop` and `/start`, plus sandbox runtime target, logs, events, preview ports, preview proxy, and terminal routes under `/v1/sandboxes/{id}`.
- The web console is a separate Vite app under `web/`; it is not embedded in the Go server.
- The API server defaults to `127.0.0.1:18080`; the Vite dev server defaults to `127.0.0.1:5174` and proxies `/healthz` and `/v1/*` to the API target, including WebSocket upgrades for sandbox terminal sessions.
- `scripts/dev.sh` is the preferred local stack entrypoint. Use `--runtime` to enable Kubernetes reconciliation/access and `--no-docker` when reusing an existing Postgres through `DATABASE_URL`.
- The runtime controller is disabled by default. It may write Kubernetes resources only when `MBOX_RUNTIME_CONTROLLER_ENABLED=true`.
- Runtime access is separately disabled by default. Terminal, logs, events, runtime target, and preview port routes require `MBOX_RUNTIME_ACCESS_ENABLED=true`.
- When enabled, the controller projects mbox sandboxes into `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources and keeps Postgres as the product source of truth.
- Sandbox ServiceAccounts and generated pod templates disable service account token automount by default.
- Project, template, and sandbox create endpoints can derive `slug` from `name` when omitted. Normal sandbox launch should rely on the project namespace and default `mbox-sandbox` ServiceAccount instead of asking users for those machine fields.
- Templates with `storageRequest` project a `workspace` PVC template mounted at the template `workingDir`, defaulting to `/workspace`; runtime target responses include resolved PVC storage metadata when available.
- EnvironmentTemplate `metadata` stores product-library presentation fields: `runtimeType`, `useCase`, `resourcePreset`, and `validationStatus`. Runtime projection must still use concrete fields such as image, startup command, resources, storage, ports, env, secret refs, network policy, and lifecycle policy.
- The template create/edit dialog should feel like a ready-to-run environment library, not a raw Kubernetes parameter sheet. Keep Essentials first: scope/name/alias, runtime, resource preset, use case, entrypoints, and workspace storage. Keep image, startup command, working directory, CPU/memory, env, secret refs, network policy, and lifecycle JSON under Advanced settings.
- The template creation dialog defaults to a practical Node.js web app workspace: `node:22-bookworm-slim`, `/workspace`, `250m`, `512Mi`, `2Gi`, and `web:3000`.
- Template entrypoint parsing should reject invalid ports instead of silently dropping them. Manual CPU/memory edits should make the saved `metadata.resourcePreset` reflect the final concrete requests, usually `Custom` when they do not match a preset.
- Sandbox ports are initialized from template `exposedPorts` and can be manually added or removed from the Runtime Workspace Preview tab. Only declared TCP ports on running sandboxes are previewable through the API proxy.
- Stopping a sandbox is a direct row action with no confirmation dialog; it marks the mbox record `stopped`, keeps `runtimeRef`, and the controller scales the resolved `agent-sandbox` `Sandbox` to zero replicas. Starting marks it `pending` and scales the existing runtime back to one replica.
- The browser terminal uses Kubernetes `pods/exec` through the resolved `agent-sandbox` Pod. The terminal route only accepts running sandboxes and only allows `sh` or `bash`.
- A newly launched sandbox can stay `pending` before the runtime exists. The Runtime Workspace should show the starting state, poll sandbox status, and avoid calling terminal/logs/events/preview runtime routes until the sandbox is `running` with a `runtimeRef`.
- The dedicated local smoke target is `MBOX_KUBECONFIG=$HOME/.kube/config` with `MBOX_KUBE_CONTEXT=kind-agent-sandbox`; this cluster is available for mbox runtime smoke tests.

## UI Guidance

The web console should feel like an operational platform:

- dense but readable
- direct status visibility
- fast repeated workflows
- clear resource and permission state
- no marketing-style landing page as the main app surface

Use cards for repeated records and modals, not for every page section. Prefer tables, split panes, detail panels, tabs, and structured forms for operational workflows.

Treat terminal as a primary workspace surface. In the current web console, selected sandboxes open a main-area Runtime Workspace with Terminal, Storage, Preview, Logs, and Events tabs; the right detail pane is metadata-only.

Core UI areas:

- Projects
- Sandboxes
- Templates
- Pipelines
- Deployments
- Policies
- Credentials
- Admin / Settings

Current UI implementation:

- `web/src/app.tsx` owns top-level console composition, active view state, and selection cleanup.
- `web/src/app.css` owns app-level layout and design tokens.
- Console shell components live in `web/src/components/console/`.
- Resource tables and dialogs live in `web/src/features/resources/`.
- Runtime Workspace panels live in `web/src/features/runtime/`.
- API wrappers, resource helpers, and data state live in `web/src/lib/` and `web/src/hooks/`.
- shadcn source components live in `web/src/components/ui/`.
- Toasts use the local shadcn Sonner wrapper in `web/src/components/ui/sonner.tsx`; do not reintroduce custom raw toast UI for row mutations.
- Keep the Notion-adjacent operational style documented in `DESIGN.md`.
- The left rail is view switching for Projects, Templates, and Sandboxes, not in-page anchor navigation. Keep only the active resource table visible.
- A selected sandbox opens the main-area Runtime Workspace only while the Sandboxes view is active. Switching views should not leave stale sandbox runtime UI visible.
- Sandbox launch requires at least one project and one template. The launch dialog should stay focused on Project, Template, and Name; slug, namespace, and ServiceAccount are derived/defaulted. Sandbox stop/start should stay in compact row actions; stop is direct, while sandbox deletion must stay behind an explicit confirmation.
- The rail active state uses soft runtime green, not white. The brand mark is the compact abstract grid mark, not a serif lowercase letter tile.

## Security Expectations

Any implementation involving runtime execution must answer:

- Which namespace does it run in?
- Which service account does it use?
- Which secrets can it read?
- Which network destinations can it reach?
- Which resources can it create, update, or delete?
- How is it cleaned up?
- Where are logs and events visible?

Do not treat these as later details.

## Verification Expectations

For runtime changes, verify with real Kubernetes resources when feasible:

- resource creation
- pod status
- logs
- events
- RBAC denial paths
- cleanup behavior

For frontend changes, run the local app and inspect it in the Codex in-app browser when the route is known.

For pipeline changes, include at least:

- success path
- failing command path
- cancellation path
- cleanup path
