# Information Architecture

This document captures the current web-console information architecture. It is intentionally narrower than the long-term product navigation in `PRODUCT.md`.

## Current App Shell

The current console is a state-switched operational surface:

- left rail
- main workspace
- right detail pane

The left rail switches the active resource view:

- Projects
- Templates
- Sandboxes

The main workspace starts with the active view title, active-view record count, and global API/product summary counts. It then shows the selected sandbox Runtime Workspace only when the active view is Sandboxes and a sandbox is selected. Below that, it renders exactly one active resource table, not all resource tables stacked on the same page.

The right detail pane is metadata-only. It shows the selected project, template, or sandbox identity and key fields. It should not host the terminal. Changing the active view clears incompatible selection so the detail pane does not show metadata from another view.

## Main Workspace Sections

### Summary

Shows current counts:

- Projects
- Templates
- Sandboxes
- Running

### Runtime Workspace

Appears when the active view is Sandboxes and the selected resource is a sandbox.

Tabs:

- Terminal
- Storage
- Preview
- Logs
- Events

Terminal is the primary operation entry for a running sandbox. Storage shows resolved workspace PVC mount path, claim, bound phase, capacity, and storage class when available. Preview lists declared TCP ports and opens API-proxied links. Logs and Events expose lightweight runtime observability.

If runtime access is blocked because the sandbox has no runtime reference or is not `running`, the workspace shows a starting/blocker state and keeps runtime actions disabled. New sandboxes can be `pending` while `agent-sandbox` creates the `SandboxClaim` and Pod; that state belongs in the workspace instead of surfacing as a terminal error.

### Projects

Current operations:

- list projects
- create project
- inspect selected project metadata

### Templates

Current operations:

- list templates
- create template
- start from a Node.js workspace default template
- capture exposed ports with entries such as `web:3000`
- inspect selected template metadata

### Sandboxes

Current operations:

- list sandboxes
- launch sandbox after a project and template exist, using only Project, Template, and Name in the user-facing dialog
- inspect selected sandbox metadata
- stop and start sandbox runtime compute from compact row actions
- delete sandbox through a confirmation dialog
- open runtime workspace for the selected sandbox
- add or remove declared TCP preview ports from the Preview tab

Stop is a direct action because it pauses runtime compute without deleting the product record. Delete remains confirmation-gated because it removes the sandbox from normal lists and triggers runtime cleanup.

## Future Navigation Areas

These are product concepts but not implemented screens yet:

- Pipelines
- Deployments
- Policies
- Credentials
- Admin / Settings

Do not add empty navigation entries for these until there is useful implemented behavior behind them.
