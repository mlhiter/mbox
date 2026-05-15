# PRODUCT

## One-liner

mbox is a Kubernetes sandbox and CI/CD control plane where people can configure, enter, run, deploy, and govern cloud execution environments.

## Product Positioning

mbox is a human-first sandbox platform that automation can also use.

The product should feel like an operational console for real execution environments:

- Create a sandbox.
- Choose or edit an environment template.
- Open terminal, IDE, notebook, browser, or preview ports.
- Run test/build/deploy pipelines.
- Inspect logs, resources, runtime status, and Kubernetes events.
- Control credentials, network access, resource limits, and cleanup policies.

The product should make Kubernetes-backed execution understandable and configurable without forcing every user to write raw Kubernetes manifests.

## Target Users

### Developers

Developers need a ready-to-run environment for a project without configuring a local machine. They care about fast startup, familiar tools, terminal access, IDE access, preview URLs, and predictable persistence.

### Platform Engineers

Platform engineers need reusable templates, quotas, policy enforcement, credential boundaries, namespace isolation, cleanup rules, observability, and integration with Kubernetes-native primitives.

### Release / DevOps Users

Release and DevOps users need repeatable pipelines, image builds, deployment targets, rollback controls, runtime logs, deployment status, and environment-specific credentials.

### Automation Clients

CI runners, scheduled jobs, and external automation can call the same runtime APIs. They are clients of the platform.

## Core Concepts

### Project

A project represents a codebase or deployable application. It binds together a Git repository, default environment templates, registry settings, deployment targets, and access policy.

### Environment Template

An environment template defines how a sandbox should start:

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

Templates should be reusable but still editable by humans through the UI.

### Sandbox

A sandbox is a runnable workspace backed by Kubernetes. It can be short-lived or long-lived, interactive or automation-driven.

Expected access modes:

- terminal
- web IDE
- Jupyter or notebook UI
- browser/computer-use style environment
- exposed preview ports
- logs and shell command history

### Pipeline

A pipeline is a configured sequence of execution steps, usually test, build, push, deploy, verify, and rollback.

Pipelines should be understandable by people and runnable by automation. The product should support a UI editor first, while keeping an exportable declarative representation.

### Deployment Target

A deployment target represents where an application can be deployed:

- preview namespace
- staging namespace
- production namespace
- external cluster

Each target has its own RBAC, secrets, network access, and approval policy.

### Policy

Policy defines what users and automation can do:

- who can create sandboxes
- who can edit templates
- who can access secrets
- who can push images
- who can deploy to a target
- which resources are allowed
- which network destinations are allowed
- when sandboxes expire

## Primary Workflows

### Create and Use a Sandbox

1. User selects a project.
2. User selects an environment template.
3. mbox creates a Kubernetes-backed sandbox.
4. User opens terminal, IDE, notebook, or preview port.
5. User edits, tests, and observes runtime state.
6. User stops, pauses, resumes, or deletes the sandbox.

### Configure an Environment Template

1. Platform user creates a template from image or existing template.
2. User configures resources, storage, ports, startup command, and tools.
3. User attaches allowed secrets and network policy.
4. User validates the template by launching a sandbox.
5. Template becomes available to selected projects or teams.

### Run a CI/CD Pipeline

1. User selects a pipeline for a project.
2. Pipeline creates controlled execution runtime.
3. Steps run tests, build image, push image, deploy, and verify.
4. User views step logs and runtime status.
5. User retries, cancels, rolls back, or promotes output.

### Deploy an Environment

1. User selects a deployment target.
2. User chooses image, config, and release parameters.
3. mbox applies deployment using target-scoped permissions.
4. User observes rollout, service status, events, and logs.
5. User can rollback from the UI.

## Product Limits

- Ordinary sandboxes must not receive cluster-admin credentials.
- Raw Kubernetes YAML must not be the only product interface.
- Operators who need Kubernetes primitives should still be able to inspect them.
- Product APIs must not be coupled permanently to one sandbox implementation.

## Product Principles

- Human-first configuration, automation-friendly API.
- Kubernetes-native under the hood, operationally legible in the UI.
- Namespace-scoped isolation by default.
- Short-lived credentials by default.
- Policies are first-class product objects, not deployment notes.
- Sandboxes and pipelines are related but not identical.
- Runtime implementation is replaceable behind a stable product contract.
