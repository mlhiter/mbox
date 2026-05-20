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

## Node.js Preview Smoke

Use this when checking the console flow that users expect from a fresh sandbox.

1. Start the stack in runtime mode:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

2. In the web console, create or select a project, create a template using the default `Node.js Workspace` values, and launch a sandbox with only Project, Template, and Name.

3. The Runtime Workspace may show `Starting runtime` while the sandbox is `pending`. Wait for the sandbox to become `running`; the workspace polls the sandbox record while it is starting.

4. In the Terminal tab, start a Node service in the background:

```sh
cat > server.js <<'EOF'
const http = require('http')
http.createServer((req, res) => {
  res.end('hello from mbox node preview')
}).listen(3000, '0.0.0.0')
EOF

node server.js > server.log 2>&1 &
```

5. In the Preview tab, add `web` port `3000` if it is not already declared. The Preview tab saves sandbox ports through `PATCH /v1/sandboxes/{id}`. Use `Open` after the sandbox is running.

## Sandbox Stop/Start Check

Use this after changing lifecycle routes, controller reconciliation, or the sandbox row actions.

1. Start runtime mode and launch a sandbox that reaches `running`.

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

2. Record the sandbox ID and stop it through the API:

```sh
SANDBOX_ID='<sandbox-id>'
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/stop"
```

Expected result:

- API returns the sandbox with `status` set to `stopped`.
- The mbox record keeps its `runtimeRef`.
- The controller scales the resolved `agent-sandbox` `Sandbox.spec.replicas` to `0`.
- The web row shows a start action instead of a stop action.

3. Start it again:

```sh
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/start"
```

Expected result:

- API returns the sandbox with `status` set to `pending`.
- The controller scales the existing runtime `Sandbox.spec.replicas` back to `1`.
- The sandbox eventually returns to `running`.

Stop/start preserves only data that is on the persistent workspace PVC. Processes and files written to container-local paths can disappear when the Pod is removed and recreated.

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

For a newly launched sandbox, `pending` is expected. The console should show `Starting runtime` and poll status instead of trying to connect the terminal before the `runtimeRef` exists.

### Preview Port Does Not Open

Check:

- the sandbox declares the port in its `ports` field
- if the template did not declare the port in `exposedPorts`, add the TCP port in the Preview tab
- protocol is TCP
- sandbox status is `running`
- runtime access is enabled
- a process is actually listening on the target port inside the sandbox

If the terminal command looks concatenated, for example `node server.jsls`, the shell did not receive a newline or the service was started in the foreground. Start the service with a background command such as `node server.js > server.log 2>&1 &`, then run `ls`, `cat server.log`, or `curl 127.0.0.1:3000` as separate commands.

### Sandbox Stop/Start Returns 404

Check that the running API server binary includes the lifecycle routes:

```sh
curl -i -X POST http://127.0.0.1:18080/v1/sandboxes/not-a-uuid/stop
curl -i -X POST http://127.0.0.1:18080/v1/sandboxes/not-a-uuid/start
```

Expected response is `400 Bad Request` with `invalid sandbox id`. A `404 Not Found` usually means Vite is proxying to a stale API process that was started before the route was added. Restart the Go API server or restart `./scripts/dev.sh`.

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
