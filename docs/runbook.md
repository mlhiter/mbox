# Runbook

This runbook covers local development, verification, and common failures for the current mbox slice.

## One-Command Local Stack

Start Postgres in Docker, the Go API server, and the Vite console:

```sh
./scripts/dev.sh
```

Default endpoints:

- API: `http://127.0.0.1:18080`
- Web: `http://127.0.0.1:5174`
- DB: `postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable`

`scripts/dev.sh` leaves the Postgres container running when stopped. It stops only the API and web dev server.

## Runtime Mode

Use runtime mode when you want the full local flow through `agent-sandbox`:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

Runtime mode sets:

- `MBOX_RUNTIME_CONTROLLER_ENABLED=true`
- `MBOX_RUNTIME_ACCESS_ENABLED=true`
- `MBOX_KUBECONFIG=${MBOX_KUBECONFIG:-$HOME/.kube/config}`
- `MBOX_KUBE_CONTEXT=${MBOX_KUBE_CONTEXT:-kind-agent-sandbox}`

Only use runtime mode against a cluster where `agent-sandbox` is installed and test resource creation is acceptable.

## Reusing an Existing Postgres

Skip Docker Postgres and use an explicit database URL:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' ./scripts/dev.sh --no-docker
```

If local port `5432` is already used by another container, either point `DATABASE_URL` at the existing mbox database or choose another host port:

```sh
MBOX_POSTGRES_PORT=15432 ./scripts/dev.sh
```

## Manual Startup

API only:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' go run ./cmd/mbox-server
```

Web only:

```sh
cd web
npm install
npm run dev
```

If the API runs on a non-default port:

```sh
cd web
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

If the default web port is busy:

```sh
cd web
MBOX_WEB_PORT=5175 npm run dev
```

## Verification

Default verification:

```sh
go test ./...
cd web && npm run build
git diff --check
```

Postgres integration tests are opt-in and write to the configured test database:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

Runtime smoke test against the dedicated local target:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke test creates and deletes Kubernetes runtime resources. It verifies the runtime `SandboxClaim`, Pod readiness, ServiceAccount token automount, workspace PVC mount, file persistence across Pod replacement, runtime storage metadata, preview-port metadata, logs, events, and runtime cleanup. Do not run it against a cluster unless that context is explicitly intended for mbox runtime testing.

## Troubleshooting

### Docker Postgres Fails on Port 5432

Symptom:

```text
Bind for 127.0.0.1:5432 failed: port is already allocated
```

Check the owner:

```sh
docker ps --format 'table {{.Names}}\t{{.Ports}}'
lsof -nP -iTCP:5432 -sTCP:LISTEN
```

Use a different mbox Postgres port:

```sh
MBOX_POSTGRES_PORT=15432 ./scripts/dev.sh
```

Or point `DATABASE_URL` at an existing mbox database and use `--no-docker`.

### Web Terminal Does Not Connect

Check:

- sandbox status is `running`
- sandbox has a `runtimeRef`
- API was started with `MBOX_RUNTIME_ACCESS_ENABLED=true`
- Vite proxy has `ws: true` for `/v1`
- the server can resolve the runtime Pod through the configured kube context

### Preview Port Does Not Open

Check:

- the template declares the port in `exposedPorts`
- the sandbox copied that port into its `ports` field at creation time
- protocol is TCP
- sandbox status is `running`
- runtime access is enabled

### Workspace Storage Does Not Persist

Check:

- the template has `storageRequest` set
- the generated `SandboxTemplate` has `spec.volumeClaimTemplates`
- the resolved runtime Pod mounts the `workspace` PVC at the template `workingDir`
- the PVC is `Bound`
- the replacement Pod reuses the same PVC after Pod deletion

### API Is Unavailable in the Web Console

Check:

```sh
curl -fsS http://127.0.0.1:18080/healthz
```

If the API uses another address, restart Vite with `MBOX_API_PROXY_TARGET`.
