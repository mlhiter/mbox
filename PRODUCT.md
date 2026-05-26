# PRODUCT

## One-liner

mbox is a Kubernetes execution platform that provides programmable sandboxes, runtime sessions, previews, artifacts, and policy boundaries for external agents, developer tools, automation systems, and human operators.

## Product Positioning

mbox is a lower-level runtime platform, not an agent product and not primarily a CI/CD product.

External agents, IDEs, CI systems, release tools, and humans should be able to call the same product APIs to request execution environments, connect to them, run work, expose outputs, and clean them up. mbox owns the execution substrate and its safety boundaries. It does not own agent reasoning, planning, code review policy, repository workflow, or release strategy.

The product should feel like an operational console and API for real execution environments:

- Create a sandbox.
- Choose or edit an environment template.
- Open terminal, IDE, notebook, browser, command, or preview sessions.
- Run controlled commands or workload tasks.
- Inspect logs, resources, runtime status, artifacts, and Kubernetes events.
- Control credentials, network access, resource limits, and cleanup policies.

The product should make Kubernetes-backed execution understandable and configurable without forcing every user to write raw Kubernetes manifests.

## Target Users

### Developers

Developers need a ready-to-run environment for a project without configuring a local machine. They care about fast startup, familiar tools, terminal access, IDE access, preview URLs, and predictable persistence.

### External Agents and Developer Tools

Agents, IDE extensions, browser automation tools, and coding assistants need a programmable runtime substrate. They should be able to create a sandbox, connect sessions, run commands, inspect logs, open preview ports, collect artifacts, and tear down resources without mbox becoming the agent itself.

### Platform Engineers

Platform engineers need reusable templates, quotas, policy enforcement, credential boundaries, namespace isolation, cleanup rules, observability, and integration with Kubernetes-native primitives.

### Automation and Release Systems

CI systems, scheduled jobs, and release tools need controlled execution environments and visible outputs. They can use mbox as an execution substrate for test, build, preview, or deployment workflows, but those workflows should remain upper-layer clients until the platform primitives are stable.

### Automation Clients

Automation clients call the public product API. They are clients of the platform, not hidden internal actors.

## Core Concepts

### Project

A project represents a codebase, workspace, or operational scope. It binds together a repository reference, default environment templates, namespace defaults, policy, credential references, and runtime history.

### Environment Template

An environment template defines how a sandbox should start:

- runtime type and use case
- base image
- installed tools
- startup command
- working directory
- resource requests and limits
- persistent volume shape
- exposed ports
- default environment variables
- secret references
- network policy
- lifecycle policy
- validation status

Templates should be reusable but still editable by humans through the UI. The first screen should sell the launch intent: runtime, use case, entrypoints, resource/storage fit, and whether the template has been validated. Raw image, command, policy, and JSON details remain available for platform users, but they should not be the first thing ordinary sandbox users must understand.

### Sandbox

A sandbox is a runnable workspace backed by Kubernetes. It can be short-lived or long-lived, interactive or automation-driven.

Expected access modes:

- terminal
- web IDE
- Jupyter or notebook UI
- browser/computer-use style environment
- exposed preview ports
- logs and shell command history

### Runtime Session

A runtime session is an attachment to a sandbox: terminal, IDE, notebook, browser, command channel, or other interactive stream. Sessions are how humans and external tools enter an existing environment.

Sessions should record who or what connected, which runtime was used, what access mode was requested, and when the connection ended. Session tracking is a platform primitive, not an agent identity model.

### Execution Task

An execution task is a controlled unit of work inside a sandbox or batch runtime. It may be a shell command, test command, build command, notebook cell runner, browser task, or another workload started by an external client.

Tasks should expose status, timing, command metadata, logs, exit result, and cleanup state. mbox should provide the execution record; the calling agent, CI system, or tool decides why the task was run.

### Preview

A preview is a product-level way to inspect a running service or output from a sandbox. The first implementation is declared TCP ports proxied through the mbox API. Future previews can include public URLs, browser sessions, notebooks, or rendered artifacts.

### Artifact

An artifact is an output of execution: logs, files, reports, screenshots, build outputs, images, or links. Artifacts should be attached to the runtime or task that produced them and made visible without exposing broad filesystem or cluster access.

### Policy

Policy defines what users and external clients can do:

- who can create sandboxes
- who can edit templates
- who can access secrets
- which credential references can be mounted or used
- which resources are allowed
- which network destinations are allowed
- when sandboxes expire
- how outputs and artifacts are retained

### Upper-layer Workflows

CI pipelines, deployment flows, release approvals, and agent task planners are upper-layer workflows that can be built on mbox. They are important use cases, but they should not define the base product model. mbox should expose durable execution primitives that those systems can compose.

## Primary Workflows

### Provision and Use a Runtime

1. User selects a project.
2. User selects an environment template.
3. mbox creates a Kubernetes-backed sandbox.
4. User or external client opens terminal, IDE, notebook, browser, command, or preview session.
5. User edits, tests, and observes runtime state.
6. User stops, pauses, resumes, or deletes the sandbox.

### Run a Controlled Task

1. External client chooses a project and runtime environment.
2. mbox creates or reuses a sandbox or batch runtime.
3. Client starts a command or workload task through the public API.
4. mbox records status, logs, runtime events, outputs, and exit result.
5. Client or human user inspects preview links and artifacts.
6. Runtime resources are stopped, retained, or deleted according to policy.

### Configure an Environment Template

1. Platform user creates a template from image or existing template.
2. User configures resources, storage, ports, startup command, and tools.
3. User attaches allowed secrets and network policy.
4. User validates the template by launching a sandbox.
5. Template becomes available to selected projects or teams.

### Integrate an Upper-layer Workflow

1. Agent, CI, IDE, or release system calls mbox to create a runtime.
2. The external workflow runs commands, opens sessions, or collects outputs through mbox APIs.
3. mbox enforces namespace, credential, resource, network, and lifecycle policy.
4. The external workflow owns its business logic, approval model, merge decision, or deployment strategy.

## Product Limits

- mbox must not include an agent brain, planner, reviewer, or autonomous coding loop as a product primitive.
- mbox must not become a CI/CD platform before the execution primitives are stable.
- Production deployment orchestration is an upper-layer integration, not a base MVP requirement.
- Ordinary sandboxes must not receive cluster-admin credentials.
- Raw Kubernetes YAML must not be the only product interface.
- Operators who need Kubernetes primitives should still be able to inspect them.
- Product APIs must not be coupled permanently to one sandbox implementation.

## Product Principles

- Human-legible configuration, automation-first API.
- Kubernetes-native under the hood, operationally legible in the UI.
- Namespace-scoped isolation by default.
- Short-lived credentials by default.
- Policies are first-class product objects, not integration notes.
- Sandboxes, sessions, tasks, previews, and artifacts are the base primitives.
- Agents, CI systems, and deployment tools are clients of mbox, not mbox itself.
- Runtime implementation is replaceable behind a stable product contract.
