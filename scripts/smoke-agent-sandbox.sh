#!/usr/bin/env bash
set -euo pipefail

API_URL="${MBOX_API_URL:-http://127.0.0.1:18080}"
KUBECONFIG_PATH="${MBOX_KUBECONFIG:-$HOME/.kube/config}"
KUBE_CONTEXT="${MBOX_KUBE_CONTEXT:-}"
RUN_ID="${MBOX_SMOKE_RUN_ID:-$(date +%Y%m%d%H%M%S)}"
PROJECT_SLUG="${MBOX_SMOKE_PROJECT_SLUG:-smoke-$RUN_ID}"
TEMPLATE_SLUG="${MBOX_SMOKE_TEMPLATE_SLUG:-busybox-terminal-$RUN_ID}"
SANDBOX_SLUG="${MBOX_SMOKE_SANDBOX_SLUG:-terminal-$RUN_ID}"
NAMESPACE="${MBOX_SMOKE_NAMESPACE:-mbox-smoke-$RUN_ID}"
SERVICE_ACCOUNT="${MBOX_SMOKE_SERVICE_ACCOUNT:-mbox-sandbox}"
TIMEOUT_SECONDS="${MBOX_SMOKE_TIMEOUT_SECONDS:-180}"
KEEP_RESOURCES="${MBOX_SMOKE_KEEP_RESOURCES:-false}"

kubectl_cmd=(kubectl --kubeconfig="$KUBECONFIG_PATH")
if [[ -n "$KUBE_CONTEXT" ]]; then
	kubectl_cmd+=(--context="$KUBE_CONTEXT")
fi

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 1
	fi
}

api_json() {
	local method="$1"
	local path="$2"
	local data="${3:-}"

	if [[ -n "$data" ]]; then
		curl -fsS -X "$method" "$API_URL$path" \
			-H "content-type: application/json" \
			-d "$data"
	else
		curl -fsS -X "$method" "$API_URL$path"
	fi
}

wait_claim_name() {
	local sandbox_id="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local claim_name=""

	while ((SECONDS < deadline)); do
		claim_name="$(api_json GET "/v1/sandboxes/$sandbox_id" | jq -r '.runtimeRef.name // empty')"
		if [[ -n "$claim_name" ]]; then
			printf '%s' "$claim_name"
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for mbox runtimeRef" >&2
	return 1
}

wait_api_sandbox_status() {
	local sandbox_id="$1"
	local expected="$2"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local status=""

	while ((SECONDS < deadline)); do
		status="$(api_json GET "/v1/sandboxes/$sandbox_id" | jq -r '.status')"
		if [[ "$status" == "$expected" ]]; then
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for sandbox $sandbox_id API status $expected; last status: $status" >&2
	return 1
}

wait_jsonpath() {
	local resource="$1"
	local jsonpath="$2"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local value=""

	while ((SECONDS < deadline)); do
		value="$("${kubectl_cmd[@]}" get "$resource" -n "$NAMESPACE" -o "jsonpath=$jsonpath" 2>/dev/null || true)"
		if [[ -n "$value" ]]; then
			printf '%s' "$value"
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for $resource jsonpath $jsonpath" >&2
	return 1
}

wait_deleted() {
	local resource="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))

	while ((SECONDS < deadline)); do
		if ! "${kubectl_cmd[@]}" get "$resource" -n "$NAMESPACE" >/dev/null 2>&1; then
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for $resource to be deleted" >&2
	return 1
}

wait_pod_name() {
	local selector="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local pod_name=""

	while ((SECONDS < deadline)); do
		pod_name="$("${kubectl_cmd[@]}" get pods -n "$NAMESPACE" -l "$selector" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
		if [[ -n "$pod_name" ]]; then
			printf '%s' "$pod_name"
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for pod with selector $selector" >&2
	return 1
}

cleanup_namespace() {
	if [[ "$KEEP_RESOURCES" != "true" && -n "${NAMESPACE:-}" ]]; then
		"${kubectl_cmd[@]}" delete namespace "$NAMESPACE" --ignore-not-found >/dev/null 2>&1 || true
		for _ in $(seq 1 "$TIMEOUT_SECONDS"); do
			if ! "${kubectl_cmd[@]}" get namespace "$NAMESPACE" >/dev/null 2>&1; then
				return 0
			fi
			sleep 1
		done
		echo "namespace $NAMESPACE is still terminating" >&2
	fi
}

require_command curl
require_command jq
require_command kubectl

echo "Checking mbox API at $API_URL"
api_json GET /healthz | jq -e '.status == "ok"' >/dev/null

echo "Checking Kubernetes context"
"${kubectl_cmd[@]}" get --raw=/readyz >/dev/null
"${kubectl_cmd[@]}" get crd sandboxclaims.extensions.agents.x-k8s.io >/dev/null
"${kubectl_cmd[@]}" get crd sandboxtemplates.extensions.agents.x-k8s.io >/dev/null

trap cleanup_namespace EXIT

echo "Creating mbox project $PROJECT_SLUG"
project_json="$(jq -n \
	--arg name "Smoke $RUN_ID" \
	--arg slug "$PROJECT_SLUG" \
	--arg namespace "$NAMESPACE" \
	'{name: $name, slug: $slug, defaultNamespace: $namespace}')"
project="$(api_json POST /v1/projects "$project_json")"
project_id="$(jq -r '.id' <<<"$project")"

echo "Creating terminal template $TEMPLATE_SLUG"
template_json="$(jq -n \
	--arg projectId "$project_id" \
	--arg name "BusyBox Terminal $RUN_ID" \
	--arg slug "$TEMPLATE_SLUG" \
	'{
		projectId: $projectId,
		name: $name,
		slug: $slug,
		image: "busybox:1.37",
		startupCommand: ["sh", "-c", "mkdir -p /workspace && echo mbox sandbox ready && sleep 86400"],
		workingDir: "/workspace",
		cpuRequest: "50m",
		memoryRequest: "64Mi",
		storageRequest: "1Gi",
		exposedPorts: []
	}')"
template="$(api_json POST /v1/templates "$template_json")"
template_id="$(jq -r '.id' <<<"$template")"

echo "Setting project default template"
api_json PATCH "/v1/projects/$project_id" "$(jq -n --arg templateId "$template_id" '{defaultTemplateId: $templateId}')" >/dev/null

echo "Creating sandbox $SANDBOX_SLUG with project defaults"
sandbox_json="$(jq -n \
	--arg projectId "$project_id" \
	--arg name "Terminal $RUN_ID" \
	--arg slug "$SANDBOX_SLUG" \
	'{projectId: $projectId, name: $name, slug: $slug}')"
sandbox="$(api_json POST /v1/sandboxes "$sandbox_json")"
sandbox_id="$(jq -r '.id' <<<"$sandbox")"

echo "Waiting for runtimeRef"
claim_name="$(wait_claim_name "$sandbox_id")"

echo "Waiting for SandboxClaim $claim_name"
"${kubectl_cmd[@]}" wait -n "$NAMESPACE" "sandboxclaim.extensions.agents.x-k8s.io/$claim_name" \
	--for=condition=Ready --timeout="${TIMEOUT_SECONDS}s"

sandbox_name="$(wait_jsonpath "sandboxclaim.extensions.agents.x-k8s.io/$claim_name" '{.status.sandbox.name}')"
pod_selector="$(wait_jsonpath "sandbox.agents.x-k8s.io/$sandbox_name" '{.status.selector}')"
pod_name="$(wait_pod_name "$pod_selector")"

echo "Waiting for Pod $pod_name"
"${kubectl_cmd[@]}" wait -n "$NAMESPACE" "pod/$pod_name" --for=condition=Ready --timeout="${TIMEOUT_SECONDS}s"

service_account_name="$("${kubectl_cmd[@]}" get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.spec.serviceAccountName}')"
token_automount="$("${kubectl_cmd[@]}" get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.spec.automountServiceAccountToken}')"
if [[ "$service_account_name" != "$SERVICE_ACCOUNT" ]]; then
	echo "expected pod serviceAccountName $SERVICE_ACCOUNT, got $service_account_name" >&2
	exit 1
fi
if [[ "$token_automount" != "false" ]]; then
	echo "expected pod automountServiceAccountToken=false, got $token_automount" >&2
	exit 1
fi

echo "Checking pod logs and workspace exec"
"${kubectl_cmd[@]}" logs -n "$NAMESPACE" "$pod_name" | grep -F "mbox sandbox ready" >/dev/null
"${kubectl_cmd[@]}" exec -n "$NAMESPACE" "$pod_name" -- sh -c 'echo smoke-ok > /workspace/mbox-smoke.txt && cat /workspace/mbox-smoke.txt' | grep -F "smoke-ok" >/dev/null

echo "Waiting for mbox API sandbox status running"
wait_api_sandbox_status "$sandbox_id" running

echo "Deleting sandbox through mbox API"
api_json DELETE "/v1/sandboxes/$sandbox_id" >/dev/null || true
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$claim_name"

echo "Deleting smoke project through mbox API"
api_json DELETE "/v1/projects/$project_id" >/dev/null

echo "Smoke passed"
echo "project=$project_id template=$template_id sandbox=$sandbox_id namespace=$NAMESPACE claim=$claim_name pod=$pod_name"
