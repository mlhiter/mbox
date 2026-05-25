# References

This file records external and internal references that shape mbox. It is not a dependency manifest.

## Runtime Substrate

- `kubernetes-sigs/agent-sandbox`: selected interactive runtime adapter for the MVP.
- mbox maps product-level sandboxes to `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.
- mbox should not expose `SandboxClaim` as the product API.

See `docs/research-agent-sandbox.md` for the current local research notes and version observations.

## Product Interaction References

- E2B product/docs: reference for treating sandboxes as ready-to-run environments selected by purpose and runtime rather than exposing raw infrastructure first.
- mbox should copy the product clarity, not the implementation boundary. mbox remains Kubernetes-native, namespace-scoped, and backed by Postgres product records plus `agent-sandbox` runtime projection.
- The current template library follows this reference by showing Environment, Use case, Entrypoints, Preset, and Status first, while keeping image, command, env, secret refs, network policy, and lifecycle JSON available under Advanced settings.

## Kubernetes Primitives

The runtime implementation currently depends on:

- Namespaces
- ServiceAccounts
- Pods
- Pod logs
- Pod events
- Pod exec
- Pod proxy for preview ports
- PersistentVolumeClaims for workspace storage
- `agent-sandbox` CRDs

Security assumptions:

- namespace-scoped isolation by default
- generated sandbox ServiceAccounts have token automount disabled
- generated sandbox Pod templates also disable token automount
- runtime access is explicit and separate from runtime reconciliation

## Frontend References

The console design direction is captured in `DESIGN.md`.

The implemented layout follows mature operational tools where terminal is a workspace-level surface:

- VS Code style bottom/main terminal surface
- GitHub Codespaces style workspace terminal
- JetBrains style expandable terminal tool window

For mbox this translates to a main-area Runtime Workspace with Terminal, Storage, Preview, Logs, and Events tabs. The right pane remains metadata-only.

## Local Tooling References

- Go API server: `cmd/mbox-server`
- Vite console: `web/`
- Preferred local stack script: `scripts/dev.sh`
- Runtime smoke script: `scripts/smoke-agent-sandbox.sh`
- API contract: `docs/server-api.md`
- Web console guide: `docs/web-console.md`
