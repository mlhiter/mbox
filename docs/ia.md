# Information Architecture

This document captures the current web-console information architecture. It is intentionally narrower than the long-term product navigation in `PRODUCT.md`.

## Current App Shell

The current console is a single-page operational surface:

- left rail
- main workspace
- right detail pane

The left rail links to the current resource sections:

- Projects
- Templates
- Sandboxes

The main workspace starts with API/product summary counts, then shows the selected sandbox Runtime Workspace when a sandbox is selected, followed by the resource tables.

The right detail pane is metadata-only. It shows the selected project, template, or sandbox identity and key fields. It should not host the terminal.

## Main Workspace Sections

### Summary

Shows current counts:

- Projects
- Templates
- Sandboxes
- Running

### Runtime Workspace

Appears when the selected resource is a sandbox.

Tabs:

- Terminal
- Preview
- Logs
- Events

Terminal is the primary operation entry for a running sandbox. Preview lists declared TCP ports and opens API-proxied links. Logs and Events expose lightweight runtime observability.

### Projects

Current operations:

- list projects
- create project
- inspect selected project metadata

### Templates

Current operations:

- list templates
- create template
- capture exposed ports with entries such as `web:3000`
- inspect selected template metadata

### Sandboxes

Current operations:

- list sandboxes
- launch sandbox
- inspect selected sandbox metadata
- delete sandbox
- open runtime workspace for the selected sandbox

## Future Navigation Areas

These are product concepts but not implemented screens yet:

- Pipelines
- Deployments
- Policies
- Credentials
- Admin / Settings

Do not add empty navigation entries for these until there is useful implemented behavior behind them.
