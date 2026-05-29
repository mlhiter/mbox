# Web Console

This document describes the currently implemented mbox web console. It is a separate Vite React app under `web/`, not a static bundle embedded in the Go API server.

## Current Scope

The current console supports the first product slice:

- API health check through `/healthz`
- view-switched left navigation for Projects, Environments, and Sandboxes
- hash-routable console locations for `#projects`, `#environments`, `#sandboxes`, and `#sandboxes/{sandboxID}`
- project list and create dialog
- template library for ready-to-run environments, with create/edit dialogs that foreground runtime type, use case, entrypoints, resource preset, and workspace storage
- advanced template settings for image, startup command, working directory, CPU, memory, env, secret refs, network preset, and lifecycle JSON
- sandbox list, simplified guarded launch dialog, stop/start actions, and delete confirmation dialog
- selected resource inspection panel for resource identity and metadata
- dedicated sandbox detail page with a main workspace runtime panel and local inspector
- browser terminal for ready sandboxes, with a starting state while new runtimes are pending
- compact workspace storage state showing resolved PVC mounts and capacity
- preview port list with manual add/remove controls and API-proxied open links for declared TCP ports
- execution task tab for running asynchronous commands in a ready sandbox, polling task state, canceling active tasks, and inspecting output
- artifact tab for registering and inspecting sandbox or task output references
- lightweight runtime logs and Kubernetes events in runtime tabs
- toast feedback for API failures and successful writes
- runtime readiness notices when terminal access is blocked by missing runtime projection or non-running sandbox status

The console does not yet provide runtime session history, artifact file transfer, credentials, or policy management. Those remain roadmap items. Pipeline and deployment screens should be treated as upper-layer integrations, not as the base console model.

## Local Development

Preferred one-command startup:

```sh
./scripts/dev.sh
```

Use runtime mode when you want the Kubernetes controller, terminal, execution tasks, logs, events, and preview proxy to exercise the real `agent-sandbox` path:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

If you already have Postgres running, provide `DATABASE_URL` and skip Docker Postgres:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' ./scripts/dev.sh --no-docker
```

Run the Go API server first:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' go run ./cmd/mbox-server
```

Then run the web console:

```sh
cd web
npm install
npm run dev
```

Default local endpoints:

- API server: `http://127.0.0.1:18080`
- Vite console: `http://127.0.0.1:5174`

Vite proxies `/healthz` and `/v1/*` to the API server. The `/v1/*` proxy also forwards WebSocket upgrades for sandbox terminal sessions. If the API server runs somewhere else, set:

```sh
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

If another local project needs the default web port, set:

```sh
MBOX_WEB_PORT=5175 npm run dev
```

## Structure

Key files:

- `web/src/app.tsx`: hash-routable top-level view state, active view copy, selection cleanup, and composition of console modules.
- `web/src/app.css`: design tokens, layout, rail, table, runtime workspace, detail pane, dialog, and confirmation styling.
- `web/src/types.ts`: shared frontend types for resources, runtime responses, selection, and forms.
- `web/src/hooks/use-mbox-data.ts`: API-backed resource loading, create/delete/start/stop mutations, selection state, counts, and toast feedback.
- `web/src/lib/api.ts`: typed fetch wrappers for `/healthz` and `/v1/*`.
- `web/src/lib/resource-utils.ts`: resource naming, runtime text, command/port parsing, storage summaries, and form cleanup helpers.
- `web/src/components/console/`: app shell, left rail, summary strip, detail pane, table state, status badges, and shared resource cells.
- `web/src/features/resources/`: project, template, and sandbox tables plus sandbox lifecycle and resource create/delete dialogs.
- `web/src/features/runtime/`: Runtime Workspace, terminal, storage, preview ports, execution tasks, logs, and events panels.
- `web/src/components/ui/`: local shadcn source components.
- `web/vite.config.ts`: Vite, Tailwind, alias, dev port, and API proxy configuration.
- `web/components.json`: shadcn project configuration.
- `docs/ia.md`: current console information architecture and future navigation boundaries.
- `docs/runbook.md`: startup, verification, and troubleshooting commands.

The app uses shadcn source components rather than a published component package. Treat these files as local code and review upstream diffs before overwriting them.

## Resource Workflows

Templates are presented as ready-to-run environments, not as a raw Kubernetes parameter sheet. The table is optimized for the questions a sandbox user asks before launch:

- What runtime is this?
- What is it for?
- How do I enter it?
- What resource/storage shape will it use?
- Has it been validated?

Template creation defaults to a usable Node.js web-app environment:

- image: `node:22-bookworm-slim`
- startup command: `sh -c 'mkdir -p /workspace && cd /workspace && echo mbox node sandbox ready && tail -f /dev/null'`
- working directory: `/workspace`
- resources: `250m` CPU, `512Mi` memory, `2Gi` storage
- exposed port: `web:3000`

The create/edit dialog has two layers:

- Essentials: scope, template name, alias, runtime, resource preset, use case, entrypoints, and workspace storage.
- Advanced settings: base image, startup command, working directory, CPU, memory, environment variables, secret references, network policy, and lifecycle policy JSON.

Runtime, use case, resource preset, and validation status are stored in template metadata. Resource presets also write the actual CPU and memory requests, so the visible product choice matches the runtime projection. If CPU or memory is manually edited away from a preset, the saved metadata preset becomes `Custom`. Invalid entrypoint ports are rejected instead of silently dropped. Editing resets `validationStatus` to `not_tested`, does not move a template between global and project scope, and does not relaunch existing sandboxes. Newly launched sandboxes inherit the saved template shape.

Sandbox launch is intentionally short. The dialog asks for Project, Template, and Name. The frontend derives the slug from the name, and the API fills namespace from the project plus the default sandbox ServiceAccount `mbox-sandbox`.

Opening a sandbox workspace moves to `#sandboxes/{sandboxID}`. The detail page is recoverable after refresh and replaces the global right detail pane with a local runtime inspector. If a detail hash is opened before data has loaded, the page shows a resolving state; if the sandbox is not present after loading, it shows an unavailable state with a route back to the Sandboxes list.

The sandbox detail page includes workspace readiness checks before the Runtime Workspace. They summarize runtime record projection, declared preview ports, workspace persistence, and run intent so users can understand the sandbox shape before they open Terminal, Preview, Tasks, Artifacts, Logs, or Events. Runtime access itself is checked inside the Runtime Workspace because local development can have a running runtime record while terminal/log/preview proxy access is disabled.

When a sandbox detail page opens, the Runtime Workspace should not treat `pending` as an error. It shows a starting panel, polls the sandbox record, and waits until the sandbox is `running` with a `runtimeRef` before calling terminal, tasks, logs, events, runtime target, or runtime preview routes.

The Preview tab edits the sandbox's declared `ports` list. A user can start a service in Terminal, add its TCP port in Preview, and open the API-proxied URL once the sandbox is running.

The Tasks tab creates controlled command tasks through `POST /v1/sandboxes/{sandboxID}/tasks`. It stays disabled until the sandbox is `running` with a `runtimeRef`, then polls active tasks, exposes cancel for `queued` and `running` tasks, and records status, command, stdout, stderr, exit result, timeout, and truncation state. The UI input is intentionally simple; API and CLI clients should use array-form commands directly, and shell behavior should be explicit through commands such as `sh -lc`.

The Artifacts tab registers output references through `POST /v1/sandboxes/{sandboxID}/artifacts` and lists the sandbox's artifact history. It can link an artifact to an existing task, but it does not upload or download file bytes in this slice. Use references such as `workspace:///workspace/out.txt`, HTTPS URLs, object-store URIs, or log/report identifiers.

## Design System

The visual direction is documented in `DESIGN.md`.

Current design principles:

- operational console, not a marketing page
- warm paper-like Notion-adjacent workspace
- restrained runtime green accent
- compact abstract grid brand mark in the rail, not a serif letter tile
- dense tables and split detail panes
- terminal as a primary workspace surface, not a narrow metadata sidebar
- runtime storage state visible beside terminal, preview, tasks, logs, and events
- cards only for repeated records and modal surfaces
- no nested cards, decorative gradients, or hero-style composition

Navigation behavior:

- The left rail uses buttons backed by lightweight hash route state, not in-page anchor jumps.
- Only the active resource table is visible in the main workspace.
- Sandbox workspace opens at `#sandboxes/{sandboxID}` and can survive browser refresh.
- Summary counts remain global, while the topbar count reflects the active view.
- Changing views clears incompatible selection so the right detail pane and Runtime Workspace cannot describe a resource from another view.
- The active nav state uses a soft runtime-green background and green ink. Do not use a white active state.

Dialog sizing is intentionally constrained in `web/src/components/ui/dialog.tsx` and reinforced by `.dialog-content` / `.dialog-grid` in `web/src/app.css`. Avoid reintroducing global utility-like class names such as `.grid`; they can collide with Tailwind utility classes and affect shadcn internals.

## Verification

Default frontend verification:

```sh
cd web
npm run build
```

When changing UI behavior, also run the local app and inspect the relevant route in the Codex in-app browser or an equivalent browser session:

```sh
cd web
npm run dev
```

Useful manual checks:

- `http://127.0.0.1:5174/` loads the console.
- API status shows healthy when the Go server is running.
- Project, template, and sandbox create dialogs fill the modal width on desktop and mobile.
- Templates table shows Environment, Use case, Entrypoints, Preset, and Status rather than leading with raw image/resource fields.
- Template create/edit opens from the Templates row action with Essentials visible and Advanced settings collapsed by default.
- Runtime selection updates the default image, use case, and entrypoints for new templates; resource preset updates the saved CPU and memory requests.
- Manual CPU/memory edits save `metadata.resourcePreset` as `Custom` when the values do not match Small, Medium, or Large.
- Invalid entrypoint text such as `web:abc` shows an error and does not save a template with missing ports.
- Template create/edit can still save advanced image, command, resources, ports, env, secret refs, network policy, and lifecycle JSON.
- Left rail buttons switch between Projects, Templates, and Sandboxes instead of scrolling a combined page.
- Switching away from Sandboxes clears sandbox selection and leaves any sandbox detail hash.
- Opening a sandbox workspace updates the URL to `#sandboxes/{sandboxID}`; refreshing that URL reopens the detail page after data loads.
- Sandbox detail shows workspace readiness checks for runtime record projection, preview surface, workspace persistence, and run intent above the Runtime Workspace.
- Sandbox launch is disabled until at least one project and one template exist.
- The launch dialog only asks for Project, Template, and Name.
- Sandbox deletion opens a confirmation dialog and does not delete from the row button directly.
- Sandbox stop/start is available from the sandbox row. Stop is direct because it pauses runtime compute without deleting the sandbox.
- Opening a ready sandbox workspace shows terminal, preview ports, execution tasks, artifacts, logs, and Kubernetes events as tabs, with compact storage state near the runtime target strip.
- Selecting a pending sandbox shows a starting Runtime Workspace and does not surface a terminal error.
- The compact storage state shows the workspace mount path, PVC name, bound phase, capacity, and storage class when a template has `storageRequest`.
- Terminal Connect is disabled until the sandbox has a runtime reference and `running` status, with the blocker visible in the workspace notice and button title.
- The Preview tab can add and remove TCP ports, and links stay disabled until the sandbox is running.
- The Tasks tab can run a simple command such as `pwd`, shows stdout/stderr, and keeps task execution disabled until the sandbox is running.
- No page-level horizontal overflow appears on mobile.
