# mbox

mbox is a Kubernetes-native sandbox and CI/CD workspace for people and automation.

The product provides a web console and API for creating runnable development sandboxes, configuring environment templates, running CI/CD pipelines, deploying preview or staged environments, and managing the policies that make those workflows safe in a shared Kubernetes cluster.

The project is independent at the product layer. Its core language is environment, sandbox, pipeline, deployment, policy, and credential management. Automation clients can use the same runtime APIs as human users and CI processes.

Long term, mbox should have several coordinated technical surfaces:

- server side: Go API server, controllers, `agent-sandbox` integration, and Kubernetes resources
- web app: human-facing operational console
- CLI: scriptable operation for developers, CI, and platform users
- API docs: published product API contract
- SDK package: Node.js or Go package for automation clients

## Product Shape

- People can create and enter sandboxes through terminal, IDE, notebook, browser, or preview endpoints.
- Platform users can define templates for language stacks, tools, startup commands, resources, storage, network access, and lifecycle rules.
- Teams can configure CI/CD pipelines that run inside controlled Kubernetes execution environments.
- Deployments can target preview, staging, or production-like namespaces with explicit permissions.
- Operators can enforce quota, RBAC, network policy, credential boundaries, and cleanup rules.

## Core Documents

- [PRODUCT.md](PRODUCT.md): product direction, users, scope, and product limits.
- [ARCHITECTURE.md](ARCHITECTURE.md): system layers, runtime design, security boundaries, and Kubernetes integration.
- [ROADMAP.md](ROADMAP.md): staged execution plan from prototype to production platform.
- [AGENTS.md](AGENTS.md): instructions for future coding agents working in this repo.
- [docs/architecture.md](docs/architecture.md): short implementation architecture handoff for the current slice.
- [docs/server-api.md](docs/server-api.md): currently implemented server routes, config, data model, and runtime projection.
- [docs/web-console.md](docs/web-console.md): Vite console structure, local proxy behavior, UI scope, and verification.
- [docs/runbook.md](docs/runbook.md): local startup, verification, runtime smoke, and troubleshooting commands.
- [docs/ia.md](docs/ia.md): current web-console information architecture.
- [docs/references.md](docs/references.md): runtime, Kubernetes, and frontend reference notes.
- [docs/research-agent-sandbox.md](docs/research-agent-sandbox.md): notes about using `kubernetes-sigs/agent-sandbox` as the interactive sandbox runtime substrate.

## Current Status

This repository now contains the first vertical slice: a Go API server backed by Postgres for mbox product records plus a separate Vite web console.

Implemented resources:

- `Project`
- `EnvironmentTemplate`
- `Sandbox`
- Vite console views for listing and creating projects, templates, and sandboxes
- simplified sandbox launch that asks for project, template, and name while deriving slug, namespace, and ServiceAccount defaults
- sandbox stop/start actions that pause and resume the projected runtime without deleting the product record
- browser terminal, workspace storage, manually declared preview ports, and lightweight logs/events for ready sandbox runtimes

This slice persists mbox product state in Postgres. When the runtime controller is explicitly enabled, it reconciles `Sandbox` records into `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.

## Local Development

Start the full local development stack:

```sh
./scripts/dev.sh
```

This starts local Postgres, the Go API server, and the Vite web console. Open `http://127.0.0.1:5174`.

Runtime access is disabled by default so ordinary local startup does not write Kubernetes resources. To enable the `agent-sandbox` controller, terminal, logs, events, and preview proxy:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

If you already have Postgres running, skip Docker Postgres:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' ./scripts/dev.sh --no-docker
```

Start a local Postgres:

```sh
docker run --name mbox-postgres \
  -e POSTGRES_USER=mbox \
  -e POSTGRES_PASSWORD=mbox \
  -e POSTGRES_DB=mbox \
  -p 5432:5432 \
  -d postgres:17
```

Run the API server:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' go run ./cmd/mbox-server
```

The server listens on `127.0.0.1:18080` by default. Override it with `MBOX_LISTEN_ADDR`.

Run the Vite web console in a second shell:

```sh
cd web
npm install
npm run dev
```

Open `http://127.0.0.1:5174`. During local development, Vite proxies `/healthz` and `/v1/*` to the API server at `127.0.0.1:18080`. If you override `MBOX_LISTEN_ADDR`, set `MBOX_API_PROXY_TARGET` before starting Vite:

```sh
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

The web dev port defaults to `5174` to avoid colliding with other local Vite projects. Override it with `MBOX_WEB_PORT`.

The runtime controller is disabled by default so local API development does not write to a Kubernetes cluster. Enable it explicitly when you want mbox to reconcile `Sandbox` records into `agent-sandbox` resources:

```sh
export MBOX_RUNTIME_CONTROLLER_ENABLED=true
export MBOX_RUNTIME_ACCESS_ENABLED=true
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT="<context-name>"
go run ./cmd/mbox-server
```

Optional runtime settings:

- `MBOX_RUNTIME_RECONCILE_INTERVAL`: reconcile loop interval, for example `5s`.
- `MBOX_RUNTIME_ACCESS_ENABLED`: enables terminal, logs, events, and runtime target routes when set to `true`.
- `MBOX_AGENT_SANDBOX_WARM_POOL`: `agent-sandbox` warm pool policy, for example `none` or `default`.

When enabled, mbox ensures the sandbox namespace exists, creates a scoped sandbox ServiceAccount with token automount disabled, creates or updates a `SandboxTemplate`, and creates a `SandboxClaim` in that namespace. The generated pod template uses the configured sandbox ServiceAccount and also disables token automount. If the template has `storageRequest`, mbox projects a `workspace` PVC template and mounts it at the template working directory, defaulting to `/workspace`. The mbox Postgres record remains the product source of truth; Kubernetes resources are the runtime projection.

Stopping a sandbox pauses the projected runtime by scaling the resolved `agent-sandbox` `Sandbox` to zero replicas. This releases the Pod and running processes while keeping the mbox record and runtime reference. Workspace data is preserved only when it lives on the persistent workspace PVC; files written to container-local paths can be lost during stop/start. Starting a stopped sandbox marks it `pending` and scales the runtime back to one replica.

`MBOX_RUNTIME_ACCESS_ENABLED=true` enables `/v1/sandboxes/{id}/terminal`, `/runtime`, `/logs`, `/events`, and `/ports`. The terminal route is a WebSocket proxy from the browser to Kubernetes `pods/exec`; declared TCP preview ports are proxied through the mbox API server to the resolved runtime Pod. Ordinary sandbox pods still do not receive broad Kubernetes credentials.

Run tests:

```sh
go test ./...
cd web && npm run build
```

Postgres integration tests are opt-in because they write to the configured test database:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

## API Smoke Test

Create a project:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/projects \
  -H 'content-type: application/json' \
  -d '{
    "name": "Demo Project",
    "repositoryUrl": "https://github.com/example/demo",
    "defaultNamespace": "mbox-demo"
  }'
```

Create a Node.js workspace template:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/templates \
  -H 'content-type: application/json' \
  -d '{
    "name": "Node.js Workspace",
    "image": "node:22-bookworm-slim",
    "startupCommand": ["sh", "-c", "mkdir -p /workspace && cd /workspace && echo mbox node sandbox ready && tail -f /dev/null"],
    "workingDir": "/workspace",
    "cpuRequest": "250m",
    "memoryRequest": "512Mi",
    "storageRequest": "2Gi",
    "exposedPorts": [
      {"name": "web", "port": 3000, "protocol": "TCP"}
    ]
  }'
```

Create a sandbox by using the returned project and template IDs:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "<project-id>",
    "templateId": "<template-id>",
    "name": "Demo Sandbox"
  }'
```

If a project has `defaultTemplateId`, sandbox creation can use project defaults:

```sh
curl -sS -X PATCH http://127.0.0.1:18080/v1/projects/<project-id> \
  -H 'content-type: application/json' \
  -d '{"defaultTemplateId":"<template-id>"}'

curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "<project-id>",
    "name": "Demo Sandbox"
  }'
```

In these paths, mbox derives the slug from the name when `slug` is omitted, uses the project `defaultNamespace`, and defaults the sandbox ServiceAccount to `mbox-sandbox`.

List resources:

```sh
curl -sS http://127.0.0.1:18080/v1/projects
curl -sS http://127.0.0.1:18080/v1/templates
curl -sS http://127.0.0.1:18080/v1/sandboxes
```

## Runtime Smoke Test

With the server running and both `MBOX_RUNTIME_CONTROLLER_ENABLED=true` and `MBOX_RUNTIME_ACCESS_ENABLED=true`, run the cluster smoke test against a kubeconfig context that already has `agent-sandbox` installed:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke test creates a project, a BusyBox terminal template, and a sandbox through the mbox API. It then verifies the generated `SandboxClaim`, resolved `Sandbox`, ready Pod, disabled ServiceAccount token automount, pod logs, workspace exec, API status mapping, and delete cleanup path.

## Node Preview Smoke

With runtime mode enabled, launch a sandbox from the Node.js workspace template and wait for it to reach `running`. The Runtime Workspace shows a starting panel while the `SandboxClaim` and Pod are still pending, then enables Terminal, Logs, Events, Storage, and Preview.

In the terminal tab, start a service in the background:

```sh
cat > server.js <<'EOF'
const http = require('http')
http.createServer((req, res) => {
  res.end('hello from mbox node preview')
}).listen(3000, '0.0.0.0')
EOF

node server.js > server.log 2>&1 &
```

Open the Preview tab, make sure port `3000` is declared as `web`, and use `Open` after the sandbox is running. If the template did not declare the port, add it in the Preview tab; this saves the sandbox `ports` field through `PATCH /v1/sandboxes/{id}`.
