#!/usr/bin/env node
import { spawnSync } from "node:child_process"

const expected = new Set([
  "README.md",
  "dist/contract.d.ts",
  "dist/contract.js",
  "dist/index.d.ts",
  "dist/index.js",
  "package.json",
])

const forbiddenPrefixes = ["src/", "scripts/", "tsconfig.json"]

const result = spawnSync("npm", ["pack", "--dry-run", "--json"], {
  encoding: "utf8",
})

if (result.status !== 0) {
  process.stderr.write(result.stderr || result.stdout)
  process.exit(result.status ?? 1)
}

let pack
try {
  pack = JSON.parse(result.stdout)[0]
} catch (error) {
  console.error(`failed to parse npm pack output: ${error instanceof Error ? error.message : String(error)}`)
  process.exit(1)
}

const files = new Set((pack?.files ?? []).map((file) => file.path))
const missing = [...expected].filter((path) => !files.has(path))
const forbidden = [...files].filter((path) => forbiddenPrefixes.some((prefix) => path === prefix || path.startsWith(prefix)))

if (missing.length > 0 || forbidden.length > 0) {
  if (missing.length > 0) {
    console.error(`npm pack is missing expected files: ${missing.join(", ")}`)
  }
  if (forbidden.length > 0) {
    console.error(`npm pack includes source-only files: ${forbidden.join(", ")}`)
  }
  process.exit(1)
}

console.log(
  `npm pack dry-run passed: ${pack.files.length} files, entry ${pack.name}@${pack.version}, archive ${pack.filename}`,
)
