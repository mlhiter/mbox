# @mbox/sdk

TypeScript SDK for the mbox HTTP API. The SDK is a thin client over product primitives: projects, templates, sandboxes, runtime access, execution tasks, preview ports, and artifact references.

It is intended for external agents, IDE tools, CI systems, release tools, and scripts that call mbox as a lower-level execution platform. It does not include an agent brain or workflow engine.

## Usage

```ts
import { MboxClient } from "@mbox/sdk"

const mbox = new MboxClient({
  baseUrl: "http://127.0.0.1:18080",
})

const sandbox = await mbox.getSandbox("sandbox-id")

const task = await mbox.createExecutionTask(sandbox.id, {
  command: ["sh", "-lc", "npm test -- --reporter=json > /workspace/reports/test.json"],
  timeoutSeconds: 300,
  metadata: { caller: "agent" },
})

const finished = await mbox.waitForTask(task.id, {
  intervalMs: 1500,
  timeoutMs: 360_000,
})

await mbox.createArtifact(sandbox.id, {
  taskId: finished.id,
  kind: "report",
  name: "test report",
  uri: "workspace:///workspace/reports/test.json",
  contentType: "application/json",
})
```

Task commands are array-form commands. Use an explicit shell such as `["sh", "-lc", "..."]` when shell parsing is required.

## Build

```sh
npm run build
```
