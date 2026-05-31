#!/usr/bin/env node
import { readFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { assertOpenAPIAlignment } from "../dist/index.js"

const source =
  process.argv[2] ??
  process.env.MBOX_OPENAPI_SOURCE ??
  process.env.MBOX_API_URL ??
  "http://127.0.0.1:18080"

try {
  const document = await loadOpenAPI(source)
  const result = assertOpenAPIAlignment(document)
  const pathCount = Object.keys(document.paths ?? {}).length
  console.log(
    `OpenAPI alignment passed: ${result.checked} SDK route entries, ${result.checkedQueryParams} SDK-used query parameters, ${result.checkedAuth} SDK route auth contracts, ${result.checkedRequests} SDK helper request contracts, ${result.checkedResponses} SDK helper response contracts, ${result.checkedSchemas} SDK schema contracts, ${result.checkedSchemaRequired} required fields, and ${result.checkedSchemaProperties} schema properties covered by ${pathCount} paths.`,
  )
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
}

async function loadOpenAPI(source) {
  if (source === "-") {
    return JSON.parse(await readStdin())
  }

  if (isHTTPURL(source)) {
    const url = openAPIURL(source)
    const response = await fetch(url, { headers: openAPIHeaders() })
    if (!response.ok) {
      throw new Error(`failed to fetch ${url}: ${response.status} ${response.statusText}`)
    }
    return response.json()
  }

  const text = await readFile(filePath(source), "utf8")
  return JSON.parse(text)
}

function openAPIURL(source) {
  const url = new URL(source)
  if (url.pathname.endsWith(".json")) {
    return url
  }
  url.pathname = `${url.pathname.replace(/\/+$/, "")}/v1/openapi.json`
  url.search = ""
  return url
}

function isHTTPURL(value) {
  try {
    const url = new URL(value)
    return url.protocol === "http:" || url.protocol === "https:"
  } catch {
    return false
  }
}

function filePath(source) {
  if (source.startsWith("file:")) {
    return fileURLToPath(source)
  }
  return source
}

function openAPIHeaders() {
  const token = process.env.MBOX_TOKEN ?? process.env.MBOX_API_TOKEN
  return token ? { authorization: `Bearer ${token}` } : undefined
}

function readStdin() {
  return new Promise((resolve, reject) => {
    let text = ""
    process.stdin.setEncoding("utf8")
    process.stdin.on("data", (chunk) => {
      text += chunk
    })
    process.stdin.on("end", () => resolve(text))
    process.stdin.on("error", reject)
  })
}
