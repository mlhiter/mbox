# Research: kubernetes-sigs/agent-sandbox

## Summary

`kubernetes-sigs/agent-sandbox` is a Kubernetes SIG Apps project that provides CRDs and a controller for isolated, stateful, singleton workloads. Its core abstraction is a sandbox: a long-running, pod-backed runtime with stable identity and optional persistent storage.

mbox uses it as the interactive sandbox runtime layer, but it should not define the entire mbox product model.

## Relevant Capabilities

- `Sandbox` custom resource.
- `SandboxTemplate` for reusable runtime configuration.
- `SandboxClaim` for requesting sandboxes from templates.
- `SandboxWarmPool` for faster startup.
- Support for stateful singleton workloads.
- Support for persistent storage.
- Potential integration with stronger isolation runtimes such as gVisor or Kata.
- SDKs for automation clients.

## Why mbox Uses It

mbox needs a Kubernetes-native way to manage interactive, stateful runtime environments.

`agent-sandbox` already focuses on:

- singleton runtime identity
- lifecycle management
- template-based creation
- long-running runtime environments
- sandbox-oriented Kubernetes APIs

These align well with mbox `Sandbox` and `EnvironmentTemplate` concepts.

## Where mbox Must Stay Separate

mbox should own:

- product API
- web console
- user and project model
- template UX
- runtime sessions
- execution tasks
- previews and artifacts
- upper-layer integration boundaries for CI and deployment clients
- policy and credential model
- audit and observability model

Do not expose `SandboxClaim` as the only product API. Map mbox records to runtime resources through an adapter.

mbox should also stay separate from agent products. External agents can call mbox to create sandboxes, connect sessions, run tasks, inspect previews, collect artifacts, and clean up resources. mbox should not contain the agent brain, planner, reviewer, or autonomous coding loop.

## Kubernetes Version Notes

Current observed `agent-sandbox` release during planning:

- latest release checked: `v0.4.6`
- release date observed: 2026-05-14
- CRDs use `apiextensions.k8s.io/v1`
- Go dependencies include `k8s.io/api v0.35.4`, `client-go v0.35.4`, and `controller-runtime v0.23.3`

Practical recommendation:

- Kubernetes 1.35 is the closest API match for the checked release.
- Kubernetes 1.34+ is a reasonable production target to investigate.
- Older clusters may accept the CRDs but fail on newer PodSpec fields or runtime behavior.

Before relying on a target cluster version, run install and lifecycle tests against that cluster version.

## Integration Shape

mbox internal runtime contract:

```text
CreateRuntime
StartRuntime
StopRuntime
PauseRuntime
ResumeRuntime
RunCommand
StreamLogs
ExposePort
AttachVolume
DeleteRuntime
GetRuntimeStatus
```

Runtime mapping:

- mbox `EnvironmentTemplate` -> `SandboxTemplate`
- mbox `Sandbox` -> `SandboxClaim` plus resolved `Sandbox`
- mbox runtime status -> Sandbox, Pod, PVC, Service, Gateway, and Events
- mbox browser terminal -> resolved Pod through Kubernetes `pods/exec`

Current implemented runtime resolution path:

1. mbox `Sandbox.runtimeRef`
2. `SandboxClaim.status.sandbox.name`
3. `Sandbox.status.selector`
4. matching Pod, preferring the `workspace` container when present

Runtime access is intentionally separate from runtime reconciliation. `MBOX_RUNTIME_CONTROLLER_ENABLED=true` controls Kubernetes resource projection, while `MBOX_RUNTIME_ACCESS_ENABLED=true` controls terminal, execution tasks, logs, events, and runtime target API routes.

## Open Questions

- Does `agent-sandbox` support all required pause/resume semantics for long-lived human sandboxes?
- Should command execution remain on Kubernetes `pods/exec` for the MVP, or move to a sidecar/gateway when authentication, auditing, or multiplexing requirements grow?
- How should web IDE, notebook, and preview ports be exposed?
- Which execution tasks should use `agent-sandbox` and which should stay as plain Kubernetes Jobs?
- How mature are warm pools for production startup latency requirements?
- How should PVC lifecycle and cleanup be coordinated between mbox and the runtime controller?
