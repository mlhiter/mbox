#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://mbox:mbox@127.0.0.1:32768/mbox?sslmode=disable}"
MBOX_API_URL="${MBOX_API_URL:-http://127.0.0.1:18080}"
MBOX_REQUEST_ID="${MBOX_REQUEST_ID:-cli-smoke-request}"
CLI=(go run ./cmd/mbox --api-url "$MBOX_API_URL" --request-id "$MBOX_REQUEST_ID" --audit-actor cli-smoke --audit-source mbox-cli)
api_pid=""
auth_api_pid=""
started_api=false

cleanup() {
	local exit_code=$?
	if [[ -n "${validation_sandbox_id:-}" ]]; then
		"${CLI[@]}" sandboxes delete "$validation_sandbox_id" >/dev/null 2>&1 || true
	fi
	if [[ -n "${sandbox_id:-}" ]]; then
		"${CLI[@]}" sandboxes delete "$sandbox_id" >/dev/null 2>&1 || true
	fi
	if [[ -n "${denied_template_id:-}" ]]; then
		curl -fsS -X DELETE "$MBOX_API_URL/v1/templates/$denied_template_id" >/dev/null 2>&1 || true
	fi
	if [[ -n "${credential_id:-}" ]]; then
		"${CLI[@]}" credentials delete "$credential_id" >/dev/null 2>&1 || true
	fi
	if [[ -n "${project_id:-}" ]]; then
		"${CLI[@]}" projects delete "$project_id" >/dev/null 2>&1 || true
	elif [[ -n "${template_id:-}" ]]; then
		curl -fsS -X DELETE "$MBOX_API_URL/v1/templates/$template_id" >/dev/null 2>&1 || true
	fi
	if [[ "$started_api" == "true" && -n "$api_pid" ]]; then
		kill "$api_pid" >/dev/null 2>&1 || true
	fi
	if [[ -n "$auth_api_pid" ]]; then
		kill "$auth_api_pid" >/dev/null 2>&1 || true
	fi
	exit "$exit_code"
}
trap cleanup EXIT INT TERM

wait_api() {
	local deadline=$((SECONDS + 30))
	while ((SECONDS < deadline)); do
		if "${CLI[@]}" health >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "timed out waiting for mbox API at $MBOX_API_URL" >&2
	return 1
}

wait_auth_api() {
	local auth_url="$1"
	local deadline=$((SECONDS + 30))
	while ((SECONDS < deadline)); do
		if curl -fsS "$auth_url/healthz" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "timed out waiting for authenticated mbox API at $auth_url" >&2
	return 1
}

cd "$ROOT_DIR"

if ! "${CLI[@]}" health >/dev/null 2>&1; then
	echo "Starting local API for CLI smoke at $MBOX_API_URL"
	MBOX_RUNTIME_CONTROLLER_ENABLED=false MBOX_RUNTIME_ACCESS_ENABLED=false DATABASE_URL="$DATABASE_URL" go run ./cmd/mbox-server &
	api_pid="$!"
	started_api=true
	wait_api
fi

if [[ "$started_api" == "true" ]]; then
	echo "Checking optional API token authentication with CLI"
	auth_addr="127.0.0.1:18081"
	auth_url="http://$auth_addr"
	MBOX_RUNTIME_CONTROLLER_ENABLED=false MBOX_RUNTIME_ACCESS_ENABLED=false MBOX_LISTEN_ADDR="$auth_addr" MBOX_API_TOKEN="cli-smoke-token" DATABASE_URL="$DATABASE_URL" go run ./cmd/mbox-server &
	auth_api_pid="$!"
	wait_auth_api "$auth_url"
	go run ./cmd/mbox --api-url "$auth_url" info | jq -e '.authenticationRequired == true' >/dev/null
	if go run ./cmd/mbox --api-url "$auth_url" projects list >/tmp/mbox-cli-auth-required.out 2>&1; then
		echo "expected authenticated API to reject requests without MBOX_TOKEN" >&2
		exit 1
	fi
	grep -F "missing or invalid bearer token" /tmp/mbox-cli-auth-required.out >/dev/null
	MBOX_TOKEN="cli-smoke-token" go run ./cmd/mbox --api-url "$auth_url" projects list | jq -e '.items | type == "array"' >/dev/null
	context_config="$(mktemp)"
	jq -n --arg url "$auth_url" '{
		currentContext: "auth-smoke",
		contexts: {
			"auth-smoke": {
				apiUrl: $url,
				tokenEnv: "MBOX_CONTEXT_TOKEN",
				auditActor: "cli-context-smoke",
				auditSource: "mbox-context"
			}
		}
	}' >"$context_config"
	MBOX_CONTEXT_TOKEN="cli-smoke-token" go run ./cmd/mbox --config "$context_config" projects list | jq -e '.items | type == "array"' >/dev/null
	MBOX_CONTEXT_TOKEN="cli-smoke-token" go run ./cmd/mbox --config "$context_config" context current | jq -e '.name == "auth-smoke" and .hasToken == true and .apiUrl == "'"$auth_url"'"' >/dev/null
	MBOX_CONTEXT_TOKEN="cli-smoke-token" go run ./cmd/mbox --config "$context_config" context list | jq -e '.items[] | select(.name == "auth-smoke" and .current == true and .hasToken == true)' >/dev/null
	context_manage_config="$(mktemp)"
	MBOX_CONTEXT_TOKEN="cli-smoke-token" go run ./cmd/mbox --config "$context_manage_config" context set managed --api-url "$auth_url" --token-env MBOX_CONTEXT_TOKEN --audit-actor cli-managed --audit-source mbox-context --current |
		jq -e '.name == "managed" and .hasToken == true and .auditActor == "cli-managed"' >/dev/null
	if grep -F "cli-smoke-token" "$context_manage_config" >/dev/null; then
		echo "expected context config to store tokenEnv instead of token value" >&2
		exit 1
	fi
	MBOX_CONTEXT_TOKEN="cli-smoke-token" go run ./cmd/mbox --config "$context_manage_config" projects list | jq -e '.items | type == "array"' >/dev/null
	go run ./cmd/mbox --config "$context_manage_config" context use managed | jq -e '.currentContext == "managed"' >/dev/null
	go run ./cmd/mbox --config "$context_manage_config" context remove managed | jq -e '.removed == true' >/dev/null
	rm -f "$context_manage_config"
	rm -f "$context_config"
	kill "$auth_api_pid" >/dev/null 2>&1 || true
	auth_api_pid=""
fi

echo "Checking API info manifest with CLI"
"${CLI[@]}" info | jq -e '
	.name == "mbox" and
	.apiVersion == "v1alpha1" and
	.runtimeController.enabled == false and
	.runtimeAccess.enabled == false and
	.artifactContent.retainedContentEnabled == true and
	.artifactContent.storageProvider == "postgres" and
	(.capabilities | index("sandboxes")) and
	(.capabilities | index("openapi")) and
	(.capabilities | index("project-usage")) and
	(.capabilities | index("project-quota-policies")) and
	(.capabilities | index("artifact-client-upload")) and
	(.capabilities | index("runtime-orphan-audit")) and
	(.capabilities | index("runtime-orphan-cleanup"))
' >/dev/null
"${CLI[@]}" compat | jq -e '
	.ok == true and
	.client == "cli" and
	.clientApiVersion == "v1alpha1" and
	.serverApiVersion == "v1alpha1" and
	.minimumApiVersion == "v1alpha1"
' >/dev/null
"${CLI[@]}" compat \
	--require-capability sandboxes \
	--require-capability execution-tasks \
	--require-capability task-events \
	--require-capability artifact-client-upload | jq -e '
	.ok == true and
	(.requiredCapabilities | index("sandboxes")) and
	(.requiredCapabilities | index("execution-tasks")) and
	(.missingCapabilities | length == 0)
' >/dev/null
"${CLI[@]}" openapi | jq -e '
	.openapi == "3.1.0" and
	.info.title == "mbox API" and
	(.paths["/v1/audit-events"].get.parameters | map(.name) | index("X-Mbox-Request-ID")) and
	(.paths["/v1/audit-events"].get.parameters | map(.name) | index("requestId")) and
	(.paths["/v1/audit-events"].get.parameters | map(.name) | index("operation")) and
	(.paths["/v1/audit-events"].get.parameters | map(.name) | index("since")) and
	(.paths["/v1/audit-events"].get.parameters | map(.name) | index("until")) and
	.paths["/v1/runtime/resources"] and
	(.paths["/v1/runtime/resources"].get.parameters | map(.name) | index("kind")) and
	(.paths["/v1/runtime/orphans"].get.parameters | map(.name) | index("kind")) and
	(.components.schemas.RuntimeResourceSummary.required | index("byOwner")) and
	.components.schemas.RuntimeResourceOwner and
	.paths["/v1/projects/{projectID}/quota-policy"] and
	.paths["/v1/tasks/{taskID}/events"] and
	.components.schemas.ProjectQuotaPolicy
' >/dev/null
if "${CLI[@]}" runtime resources >/tmp/mbox-cli-runtime-resources.out 2>&1; then
	echo "expected runtime resources to require a configured runtime auditor" >&2
	exit 1
fi
grep -F "runtime auditor is not configured" /tmp/mbox-cli-runtime-resources.out >/dev/null
if "${CLI[@]}" runtime orphans >/tmp/mbox-cli-runtime-orphans.out 2>&1; then
	echo "expected runtime orphan audit to require a configured runtime auditor" >&2
	exit 1
fi
grep -F "runtime auditor is not configured" /tmp/mbox-cli-runtime-orphans.out >/dev/null
if "${CLI[@]}" runtime cleanup-orphan --kind SandboxClaim --namespace missing --name missing --reason missing-sandbox-record --confirm delete-orphan-runtime-resource >/tmp/mbox-cli-runtime-cleanup.out 2>&1; then
	echo "expected runtime orphan cleanup to require a configured runtime cleaner" >&2
	exit 1
fi
grep -F "runtime cleaner is not configured" /tmp/mbox-cli-runtime-cleanup.out >/dev/null

suffix="$(date +%Y%m%d%H%M%S)"
project_name="cli-smoke-$suffix"
template_name="cli-smoke-template-$suffix"
sandbox_name="cli-smoke-sandbox-$suffix"

echo "Creating project with CLI"
project_json="$("${CLI[@]}" projects create --name "$project_name" --slug "$project_name" --namespace "$project_name")"
project_id="$(jq -r '.id' <<<"$project_json")"

echo "Creating template through API"
template_json="$(curl -fsS -X POST "$MBOX_API_URL/v1/templates" \
	-H 'content-type: application/json' \
	-d "$(jq -n \
		--arg projectId "$project_id" \
		--arg name "$template_name" \
		--arg slug "$template_name" \
		'{
			projectId: $projectId,
			name: $name,
			slug: $slug,
			image: "busybox:1.36",
			startupCommand: ["sh", "-c", "echo mbox cli smoke ready && tail -f /dev/null"],
			workingDir: "/workspace"
		}')")"
template_id="$(jq -r '.id' <<<"$template_json")"

echo "Setting enforced project launch policy with CLI"
"${CLI[@]}" projects set-policy "$project_id" \
	--enforcement enforced \
	--allowed-image-prefix busybox: \
	--allowed-service-account mbox-sandbox | jq -e '.enforcement == "enforced" and (.allowedImagePrefixes | index("busybox:")) and (.allowedServiceAccounts | index("mbox-sandbox"))' >/dev/null
"${CLI[@]}" projects policy "$project_id" | jq -e '.enforcement == "enforced"' >/dev/null

echo "Setting project quota policy with CLI"
"${CLI[@]}" projects set-quota-policy "$project_id" \
	--enforcement enforced \
	--max-active-sandboxes 5 \
	--max-retained-artifact-bytes 1048576 | jq -e '.enforcement == "enforced" and .maxActiveSandboxes == 5 and .maxRetainedArtifactBytes == 1048576' >/dev/null
"${CLI[@]}" projects quota-policy "$project_id" | jq -e '.enforcement == "enforced" and .maxActiveSandboxes == 5 and .maxRetainedArtifactBytes == 1048576' >/dev/null

echo "Registering project credential reference with CLI"
credential_json="$("${CLI[@]}" projects add-credential "$project_id" \
	--name "CLI smoke Git" \
	--slug "cli-smoke-git-$suffix" \
	--type git \
	--target "https://github.com/mlhiter/mbox" \
	--secret-ref "cli-smoke-git-token" \
	--secret-key token \
	--usage clone \
	--usage fetch)"
credential_id="$(jq -r '.id' <<<"$credential_json")"
"${CLI[@]}" projects credentials "$project_id" | jq -e --arg id "$credential_id" '.items[] | select(.id == $id and .secretRef.name == "cli-smoke-git-token")' >/dev/null
"${CLI[@]}" credentials get "$credential_id" | jq -e --arg id "$credential_id" '.id == $id and .type == "git"' >/dev/null

echo "Checking policy denial path"
denied_template_json="$(curl -fsS -X POST "$MBOX_API_URL/v1/templates" \
	-H 'content-type: application/json' \
	-d "$(jq -n \
		--arg projectId "$project_id" \
		--arg name "cli-smoke-denied-$suffix" \
		--arg slug "cli-smoke-denied-$suffix" \
		'{
			projectId: $projectId,
			name: $name,
			slug: $slug,
			image: "ubuntu:24.04",
			startupCommand: ["sh", "-c", "tail -f /dev/null"],
			workingDir: "/workspace"
		}')")"
denied_template_id="$(jq -r '.id' <<<"$denied_template_json")"
if "${CLI[@]}" sandboxes create --project-id "$project_id" --template-id "$denied_template_id" --name "cli-smoke-denied-$suffix" --slug "cli-smoke-denied-$suffix" >/tmp/mbox-cli-policy-denied.out 2>&1; then
	echo "expected policy-denied sandbox launch to fail" >&2
	exit 1
fi
grep -F "policy denied" /tmp/mbox-cli-policy-denied.out >/dev/null
curl -fsS -X DELETE "$MBOX_API_URL/v1/templates/$denied_template_id" >/dev/null
denied_template_id=""

echo "Checking quota denial path"
"${CLI[@]}" projects set-quota-policy "$project_id" \
	--enforcement enforced \
	--max-active-sandboxes 0 \
	--max-retained-artifact-bytes 1048576 >/dev/null
if "${CLI[@]}" sandboxes create --project-id "$project_id" --template-id "$template_id" --name "cli-smoke-quota-denied-$suffix" --slug "cli-smoke-quota-denied-$suffix" >/tmp/mbox-cli-quota-denied.out 2>&1; then
	echo "expected quota-denied sandbox launch to fail" >&2
	exit 1
fi
grep -F "active sandbox quota exceeded" /tmp/mbox-cli-quota-denied.out >/dev/null
"${CLI[@]}" projects set-quota-policy "$project_id" \
	--enforcement enforced \
	--max-active-sandboxes 5 \
	--max-retained-artifact-bytes 1048576 >/dev/null

echo "Creating sandbox with CLI"
sandbox_json="$("${CLI[@]}" sandboxes create --project-id "$project_id" --template-id "$template_id" --name "$sandbox_name" --slug "$sandbox_name")"
sandbox_id="$(jq -r '.id' <<<"$sandbox_json")"

"${CLI[@]}" projects list | jq -e --arg id "$project_id" '.items[] | select(.id == $id)' >/dev/null
"${CLI[@]}" templates get "$template_id" | jq -e --arg id "$template_id" '.id == $id' >/dev/null
"${CLI[@]}" sandboxes get "$sandbox_id" | jq -e --arg id "$sandbox_id" '.id == $id and .status == "pending"' >/dev/null
"${CLI[@]}" projects usage "$project_id" | jq -e '.sandboxes.active == 1 and .sandboxes.pending == 1 and .templates.projectScoped == 1 and .credentials.total == 1 and .artifacts.total == 0' >/dev/null
"${CLI[@]}" projects audit-events "$project_id" --action sandbox.created --resource-type sandbox --actor cli-smoke --source mbox-cli --filter-request-id "$MBOX_REQUEST_ID" --limit 10 | jq -e --arg requestId "$MBOX_REQUEST_ID" '.items[] | select(.action == "sandbox.created" and .resourceType == "sandbox" and .actor == "cli-smoke" and .source == "mbox-cli" and .metadata.requestId == $requestId)' >/dev/null
"${CLI[@]}" projects audit-events "$project_id" --action policy.denied --resource-type sandbox --operation sandbox.launch --actor cli-smoke --source mbox-cli --filter-request-id "$MBOX_REQUEST_ID" --limit 10 | jq -e --arg requestId "$MBOX_REQUEST_ID" '.items[] | select(.action == "policy.denied" and .resourceType == "sandbox" and .actor == "cli-smoke" and .source == "mbox-cli" and .metadata.requestId == $requestId and .metadata.operation == "sandbox.launch")' >/dev/null
"${CLI[@]}" audit-events --project-id "$project_id" --action project.credential.created --resource-type project-credential --actor cli-smoke --source mbox-cli --limit 10 | jq -e '.items[] | select(.action == "project.credential.created" and .resourceType == "project-credential" and .actor == "cli-smoke" and .source == "mbox-cli")' >/dev/null
"${CLI[@]}" templates boundary "$template_id" --project-id "$project_id" | jq -e '.serviceAccountTokenAutomount == false and .secretProjection == "none" and .credentialProjection == "references-recorded-not-mounted" and .policyEnforcement == "enforced" and (.checks[] | select(.id == "launch-policy" and .status == "pass")) and (.checks[] | select(.id == "credential-refs" and .status == "warn"))' >/dev/null
"${CLI[@]}" sandboxes boundary "$sandbox_id" | jq -e --arg id "$sandbox_id" '.sandboxId == $id and .serviceAccountTokenAutomount == false and .credentialProjection == "references-recorded-not-mounted" and .policyEnforcement == "enforced"' >/dev/null

echo "Checking template validation commands with CLI"
validation_json="$("${CLI[@]}" templates validate "$template_id" --project-id "$project_id" --name "Validate $template_name" --metadata '{"caller":"cli-smoke"}')"
validation_sandbox_id="$(jq -r '.sandbox.id' <<<"$validation_json")"
"${CLI[@]}" templates decide-validation "$template_id" "$validation_sandbox_id" --status passed | jq -e '.template.metadata.validationStatus == "passed" and .sandbox.metadata.validationResult == "passed"' >/dev/null
"${CLI[@]}" sandboxes delete "$validation_sandbox_id" >/dev/null
validation_sandbox_id=""

echo "Checking session records with CLI"
session_json="$("${CLI[@]}" sessions create "$sandbox_id" --type custom --client cli-smoke)"
session_id="$(jq -r '.id' <<<"$session_json")"
"${CLI[@]}" sessions list "$sandbox_id" | jq -e --arg id "$session_id" '.items[] | select(.id == $id and .status == "active")' >/dev/null
"${CLI[@]}" sessions end "$session_id" | jq -e '.status == "ended" and (.endedAt | type == "string")' >/dev/null

echo "Checking client artifact upload with CLI"
artifact_json="$(curl -fsS -X POST "$MBOX_API_URL/v1/sandboxes/$sandbox_id/artifacts" \
	-H 'content-type: application/json' \
	-d "$(jq -n '{
		kind: "report",
		name: "cli-upload.txt",
		uri: "client://cli-smoke/cli-upload.txt",
		contentType: "text/plain"
	}')")"
artifact_id="$(jq -r '.id' <<<"$artifact_json")"
printf 'cli-upload-ok' | "${CLI[@]}" artifacts upload "$artifact_id" --stdin --content-type text/plain | jq -e '.retainedContent.sizeBytes == 13 and (.retainedContent.sha256 | length == 64)' >/dev/null
"${CLI[@]}" artifacts get "$artifact_id" | jq -e '.retainedContent.storageProvider == "postgres"' >/dev/null
"${CLI[@]}" artifacts content "$artifact_id" | grep -F "cli-upload-ok" >/dev/null
"${CLI[@]}" projects usage "$project_id" | jq -e '.runtimeSessions.total == 1 and .runtimeSessions.ended == 1 and .artifacts.total == 1 and .artifacts.retainedContent == 1 and .artifacts.retainedBytes == 13' >/dev/null

echo "Deleting project and template"
if [[ -n "${credential_id:-}" ]]; then
	"${CLI[@]}" credentials delete "$credential_id" >/dev/null
	credential_id=""
fi
if [[ -n "${sandbox_id:-}" ]]; then
	"${CLI[@]}" sandboxes delete "$sandbox_id" >/dev/null
	sandbox_id=""
fi
"${CLI[@]}" projects delete "$project_id" >/dev/null
project_id=""
template_id=""

echo "CLI smoke passed"
