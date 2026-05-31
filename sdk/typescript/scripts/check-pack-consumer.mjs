#!/usr/bin/env node
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { spawnSync } from "node:child_process"

const root = new URL("..", import.meta.url)
const tempDir = mkdtempSync(join(tmpdir(), "mbox-sdk-consumer-"))
const tarballDir = join(tempDir, "tarball")
const consumerDir = join(tempDir, "consumer")

try {
  mkdirSync(tarballDir)
  mkdirSync(consumerDir)
  const pack = run("npm", ["pack", "--pack-destination", tarballDir], root)
  const filename = pack.stdout.trim().split(/\r?\n/).at(-1)
  if (!filename) {
    throw new Error("npm pack did not report a tarball filename")
  }
  const tarball = join(tarballDir, filename)

  writeFileSync(
    join(consumerDir, "package.json"),
    JSON.stringify(
      {
        private: true,
        type: "module",
        dependencies: {
          "@mbox/sdk": `file:${tarball}`,
        },
      },
      null,
      2,
    ),
  )
  writeFileSync(
    join(consumerDir, "smoke.mjs"),
    `import {
  MboxClient,
  assertOpenAPIAlignment,
  checkSDKCompatibility,
} from "@mbox/sdk"

const result = checkSDKCompatibility({
  name: "mbox",
  apiVersion: "v1alpha1",
  serverVersion: "consumer-smoke",
  runtimeController: { enabled: false },
  runtimeAccess: { enabled: false },
  artifactContent: {
    retainedContentEnabled: true,
    storageProvider: "s3",
    maxBytes: 8388608,
  },
  capabilities: ["sandboxes", "execution-tasks"],
  compatibility: {
    minimumCliApiVersion: "v1alpha1",
    minimumSdkApiVersion: "v1alpha1",
  },
  authenticationRequired: false,
}, "v1alpha1", ["execution-tasks"])

if (!result.ok) {
  throw new Error(result.message)
}

if (typeof MboxClient !== "function") {
  throw new Error("MboxClient export is not constructable")
}

if (typeof assertOpenAPIAlignment !== "function") {
  throw new Error("assertOpenAPIAlignment export is missing")
}
`,
  )

  run("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund"], consumerDir)
  run("node", ["smoke.mjs"], consumerDir)
  console.log(`npm pack consumer smoke passed: ${tarball}`)
} finally {
  rmSync(tempDir, { force: true, recursive: true })
}

function run(command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
  })
  if (result.status !== 0) {
    process.stderr.write(result.stdout)
    process.stderr.write(result.stderr)
    process.exit(result.status ?? 1)
  }
  return result
}
