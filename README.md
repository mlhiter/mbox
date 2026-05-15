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
- [docs/research-agent-sandbox.md](docs/research-agent-sandbox.md): notes about using `kubernetes-sigs/agent-sandbox` as the interactive sandbox runtime substrate.

## Current Status

This repository currently contains planning documents only. Implementation should follow the selected `agent-sandbox` direction for interactive sandboxes and the product boundaries described in the product and architecture documents.
