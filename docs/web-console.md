# Web Console

This document describes the currently implemented mbox web console. It is a separate Vite React app under `web/`, not a static bundle embedded in the Go API server.

## Current Scope

The current console supports the first product slice:

- API health check through `/healthz`
- project list and create dialog
- template list and create dialog
- sandbox list and launch dialog
- selected resource inspection panel for resource identity and metadata
- main workspace runtime panel for selected sandboxes
- browser terminal for ready sandboxes
- preview port list with API-proxied open links for declared TCP ports
- lightweight runtime logs and Kubernetes events in runtime tabs
- toast feedback for API failures and successful writes

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

- `web/src/app.tsx`: current single-page console, resource tables, dialogs, and API calls.
- `web/src/app.css`: design tokens, layout, table, runtime workspace, detail pane, and dialog styling.
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
- dense tables and split detail panes
- terminal as a primary workspace surface, not a narrow metadata sidebar
- cards only for repeated records and modal surfaces
- no nested cards, decorative gradients, or hero-style composition

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
- Selecting a ready sandbox opens a main Runtime Workspace with terminal, preview ports, logs, and Kubernetes events as tabs.
- Terminal Connect is disabled until the sandbox has a runtime reference and `running` status.
- No page-level horizontal overflow appears on mobile.
