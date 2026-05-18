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
- [docs/server-api.md](docs/server-api.md): currently implemented server routes, config, data model, and runtime projection.
- [docs/research-agent-sandbox.md](docs/research-agent-sandbox.md): notes about using `kubernetes-sigs/agent-sandbox` as the interactive sandbox runtime substrate.

## Current Status

This repository now contains the first server slice: a Go API server backed by Postgres for mbox product records.

Implemented resources:

- `Project`
- `EnvironmentTemplate`
- `Sandbox`

This slice persists mbox product state in Postgres. When the runtime controller is explicitly enabled, it reconciles `Sandbox` records into `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.

## Local Development

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

Open `http://127.0.0.1:5173`. During local development, Vite proxies `/healthz` and `/v1/*` to the API server at `127.0.0.1:18080`. If you override `MBOX_LISTEN_ADDR`, set `MBOX_API_PROXY_TARGET` before starting Vite:

```sh
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

The runtime controller is disabled by default so local API development does not write to a Kubernetes cluster. Enable it explicitly when you want mbox to reconcile `Sandbox` records into `agent-sandbox` resources:

```sh
export MBOX_RUNTIME_CONTROLLER_ENABLED=true
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT="<context-name>"
go run ./cmd/mbox-server
```

Optional runtime settings:

- `MBOX_RUNTIME_RECONCILE_INTERVAL`: reconcile loop interval, for example `5s`.
- `MBOX_AGENT_SANDBOX_WARM_POOL`: `agent-sandbox` warm pool policy, for example `none` or `default`.

When enabled, mbox ensures the sandbox namespace exists, creates a scoped sandbox ServiceAccount with token automount disabled, creates or updates a `SandboxTemplate`, and creates a `SandboxClaim` in that namespace. The generated pod template uses the configured sandbox ServiceAccount and also disables token automount. The mbox Postgres record remains the product source of truth; Kubernetes resources are the runtime projection.

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
    "slug": "demo-project",
    "repositoryUrl": "https://github.com/example/demo",
    "defaultNamespace": "mbox-demo"
  }'
```

Create a template:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/templates \
  -H 'content-type: application/json' \
  -d '{
    "name": "Ubuntu Terminal",
    "slug": "ubuntu-terminal",
    "image": "ubuntu:24.04",
    "workingDir": "/workspace",
    "cpuRequest": "500m",
    "memoryRequest": "1Gi",
    "storageRequest": "10Gi",
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
    "name": "Demo Sandbox",
    "slug": "demo-sandbox",
    "namespace": "mbox-demo",
    "serviceAccountName": "mbox-sandbox"
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
    "name": "Demo Sandbox",
    "slug": "demo-sandbox"
  }'
```

In this path, mbox uses the project `defaultNamespace`, the project `defaultTemplateId`, and the default sandbox ServiceAccount `mbox-sandbox`.

List resources:

```sh
curl -sS http://127.0.0.1:18080/v1/projects
curl -sS http://127.0.0.1:18080/v1/templates
curl -sS http://127.0.0.1:18080/v1/sandboxes
```

## Runtime Smoke Test

With the server running and `MBOX_RUNTIME_CONTROLLER_ENABLED=true`, run the cluster smoke test against a kubeconfig context that already has `agent-sandbox` installed:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke test creates a project, a BusyBox terminal template, and a sandbox through the mbox API. It then verifies the generated `SandboxClaim`, resolved `Sandbox`, ready Pod, disabled ServiceAccount token automount, pod logs, workspace exec, API status mapping, and delete cleanup path.
