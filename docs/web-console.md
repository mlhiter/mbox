# Web Console

This document describes the currently implemented mbox web console. It is a separate Vite React app under `web/`, not a static bundle embedded in the Go API server.

## Current Scope

The current console supports the first product slice:

- API health check through `/healthz`
- view-switched left navigation for Projects, Templates, and Sandboxes
- project list and create dialog
- template list and create dialog
- sandbox list, guarded launch dialog, and delete confirmation dialog
- selected resource inspection panel for resource identity and metadata
- main workspace runtime panel for selected sandboxes only in the Sandboxes view
- browser terminal for ready sandboxes
- workspace storage tab showing resolved PVC mounts and capacity
- preview port list with API-proxied open links for declared TCP ports
- lightweight runtime logs and Kubernetes events in runtime tabs
- toast feedback for API failures and successful writes
- runtime readiness notices when terminal access is blocked by missing runtime projection or non-running sandbox status

The console does not yet provide pipeline editing, deployments, credentials, or policy management. Those remain roadmap items.

## Local Development

Preferred one-command startup:

```sh
./scripts/dev.sh
```

Use runtime mode when you want the Kubernetes controller, terminal, logs, events, and preview proxy to exercise the real `agent-sandbox` path:

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

- `web/src/app.tsx`: top-level view state, active view copy, selection cleanup, and composition of console modules.
- `web/src/app.css`: design tokens, layout, rail, table, runtime workspace, detail pane, dialog, and confirmation styling.
- `web/src/types.ts`: shared frontend types for resources, runtime responses, selection, and forms.
- `web/src/hooks/use-mbox-data.ts`: API-backed resource loading, create/delete mutations, selection state, counts, and toast feedback.
- `web/src/lib/api.ts`: typed fetch wrappers for `/healthz` and `/v1/*`.
- `web/src/lib/resource-utils.ts`: resource naming, runtime text, command/port parsing, storage summaries, and form cleanup helpers.
- `web/src/components/console/`: app shell, left rail, summary strip, detail pane, table state, status badges, and shared resource cells.
- `web/src/features/resources/`: project, template, and sandbox tables plus resource create/delete dialogs.
- `web/src/features/runtime/`: Runtime Workspace, terminal, storage, preview ports, logs, and events panels.
- `web/src/components/ui/`: local shadcn source components.
- `web/vite.config.ts`: Vite, Tailwind, alias, dev port, and API proxy configuration.
- `web/components.json`: shadcn project configuration.
- `docs/ia.md`: current console information architecture and future navigation boundaries.
- `docs/runbook.md`: startup, verification, and troubleshooting commands.

The app uses shadcn source components rather than a published component package. Treat these files as local code and review upstream diffs before overwriting them.

## Design System

The visual direction is documented in `DESIGN.md`.

Current design principles:

- operational console, not a marketing page
- warm paper-like Notion-adjacent workspace
- restrained runtime green accent
- compact abstract grid brand mark in the rail, not a serif letter tile
- dense tables and split detail panes
- terminal as a primary workspace surface, not a narrow metadata sidebar
- runtime storage state visible beside terminal, preview, logs, and events
- cards only for repeated records and modal surfaces
- no nested cards, decorative gradients, or hero-style composition

Navigation behavior:

- The left rail uses buttons backed by React view state, not in-page anchor jumps.
- Only the active resource table is visible in the main workspace.
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
- Left rail buttons switch between Projects, Templates, and Sandboxes instead of scrolling a combined page.
- Switching away from Sandboxes clears sandbox selection and hides the Runtime Workspace.
- Sandbox launch is disabled until at least one project and one template exist.
- Sandbox deletion opens a confirmation dialog and does not delete from the row button directly.
- Selecting a ready sandbox opens a main Runtime Workspace with terminal, storage, preview ports, logs, and Kubernetes events as tabs.
- The Storage tab shows the workspace mount path, PVC name, bound phase, capacity, and storage class when a template has `storageRequest`.
- Terminal Connect is disabled until the sandbox has a runtime reference and `running` status, with the blocker visible in the workspace notice and button title.
- No page-level horizontal overflow appears on mobile.
