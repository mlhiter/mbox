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
EXPECTED_ARTIFACT_CONTENT_BACKEND="${MBOX_ARTIFACT_CONTENT_BACKEND:-postgres}"
LIFECYCLE_TTL_SECONDS="${MBOX_SMOKE_LIFECYCLE_TTL_SECONDS:-30}"

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

api_raw() {
	local method="$1"
	local path="$2"
	curl -fsS -X "$method" "$API_URL$path"
}

cli_json() {
	go run ./cmd/mbox --api-url "$API_URL" "$@"
}

wait_claim_name() {
	local sandbox_id="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local claim_name=""
	local response=""
	local code=""

	while ((SECONDS < deadline)); do
		response="$(mktemp)"
		code="$(curl -sS -o "$response" -w '%{http_code}' -X GET "$API_URL/v1/sandboxes/$sandbox_id" || true)"
		if [[ "$code" == "404" ]]; then
			rm -f "$response"
			echo "sandbox $sandbox_id disappeared before runtimeRef was assigned" >&2
			return 1
		fi
		if [[ "$code" != "200" ]]; then
			rm -f "$response"
			sleep 2
			continue
		fi
		claim_name="$(jq -r '.runtimeRef.name // empty' <"$response")"
		rm -f "$response"
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

wait_api_sandbox_not_found() {
	local sandbox_id="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local code=""

	while ((SECONDS < deadline)); do
		code="$(curl -sS -o /dev/null -w '%{http_code}' -X GET "$API_URL/v1/sandboxes/$sandbox_id" || true)"
		if [[ "$code" == "404" ]]; then
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for sandbox $sandbox_id to be hidden after lifecycle cleanup; last HTTP code: $code" >&2
	return 1
}

wait_api_sandbox_runtime_ref_cleared() {
	local sandbox_id="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local response=""
	local runtime_ref=""
	local code=""

	while ((SECONDS < deadline)); do
		response="$(mktemp)"
		code="$(curl -sS -o "$response" -w '%{http_code}' -X GET "$API_URL/v1/sandboxes/$sandbox_id" || true)"
		if [[ "$code" == "404" ]]; then
			rm -f "$response"
			return 0
		fi
		if [[ "$code" == "200" ]]; then
			runtime_ref="$(jq -r '.runtimeRef.name // empty' <"$response")"
			rm -f "$response"
			if [[ -z "$runtime_ref" ]]; then
				return 0
			fi
		else
			rm -f "$response"
		fi
		sleep 2
	done

	echo "timed out waiting for sandbox $sandbox_id runtimeRef cleanup; last HTTP code: $code" >&2
	return 1
}

wait_task_done() {
	local task_id="$1"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local status=""

	while ((SECONDS < deadline)); do
		status="$(api_json GET "/v1/tasks/$task_id" | jq -r '.status')"
		case "$status" in
			succeeded|failed|canceled|timed_out)
				printf '%s' "$status"
				return 0
				;;
		esac
		sleep 1
	done

	echo "timed out waiting for task $task_id to finish; last status: $status" >&2
	return 1
}

wait_preview_port() {
	local sandbox_id="$1"
	local port="$2"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local needle="/ports/$port/proxy/"

	while ((SECONDS < deadline)); do
		if api_json GET "/v1/sandboxes/$sandbox_id/ports" | jq -e \
			--argjson port "$port" \
			--arg needle "$needle" \
			'.items[]? | select(.port == $port and .available == true and ((.previewUrl // "") | contains($needle)))' >/dev/null; then
			return 0
		fi
		sleep 1
	done

	echo "timed out waiting for preview port $port to become available" >&2
	api_json GET "/v1/sandboxes/$sandbox_id/ports" >&2 || true
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

wait_replacement_pod_name() {
	local selector="$1"
	local old_uid="$2"
	local deadline=$((SECONDS + TIMEOUT_SECONDS))
	local pod_name=""

	while ((SECONDS < deadline)); do
		pod_name="$("${kubectl_cmd[@]}" get pods -n "$NAMESPACE" -l "$selector" -o json 2>/dev/null | jq -r --arg old "$old_uid" '.items[] | select(.metadata.uid != $old and (.metadata.deletionTimestamp | not)) | .metadata.name' | head -n 1 || true)"
		if [[ -n "$pod_name" ]]; then
			printf '%s' "$pod_name"
			return 0
		fi
		sleep 2
	done

	echo "timed out waiting for replacement pod with selector $selector" >&2
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
api_json GET /v1/info | jq -e \
	--arg backend "$EXPECTED_ARTIFACT_CONTENT_BACKEND" \
	'.name == "mbox" and
	.apiVersion == "v1alpha1" and
	.runtimeController.enabled == true and
	.runtimeAccess.enabled == true and
	.runtimeAccess.adapter == "agent-sandbox" and
	.artifactContent.storageProvider == $backend and
	(.capabilities | index("openapi")) and
	(.capabilities | index("project-usage")) and
	(.capabilities | index("project-quota-policies")) and
	(.capabilities | index("execution-tasks")) and
	(.capabilities | index("artifact-client-upload")) and
	(.capabilities | index("runtime-orphan-audit")) and
	(.capabilities | index("runtime-orphan-cleanup"))' >/dev/null
cli_json openapi | jq -e '
	.openapi == "3.1.0" and
	.paths["/v1/sandboxes/{sandboxID}/runtime"] and
	.paths["/v1/artifacts/{artifactID}/content"] and
	.components.schemas.RuntimeResourceObservation and
	.components.schemas.RuntimeWorkloadSummary and
	.components.schemas.RuntimeStorage and
	.components.schemas.ExecutionTask
' >/dev/null

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

echo "Creating short-lived lifecycle template"
lifecycle_template_json="$(jq -n \
	--arg projectId "$project_id" \
	--arg name "TTL BusyBox $RUN_ID" \
	--arg slug "ttl-busybox-$RUN_ID" \
	--arg ttlSeconds "$LIFECYCLE_TTL_SECONDS" \
	'{
		projectId: $projectId,
		name: $name,
		slug: $slug,
		image: "busybox:1.37",
		startupCommand: ["sh", "-c", "echo mbox ttl ready && sleep 86400"],
		workingDir: "/workspace",
		lifecyclePolicy: {ttlSeconds: ($ttlSeconds | tonumber)}
	}')"
lifecycle_template="$(api_json POST /v1/templates "$lifecycle_template_json")"
lifecycle_template_id="$(jq -r '.id' <<<"$lifecycle_template")"

echo "Setting project default template"
api_json PATCH "/v1/projects/$project_id" "$(jq -n --arg templateId "$template_id" '{defaultTemplateId: $templateId}')" >/dev/null

echo "Setting enforced project launch policy"
api_json PUT "/v1/projects/$project_id/policy" "$(jq -n \
	--arg imagePrefix "busybox:" \
	--arg serviceAccount "$SERVICE_ACCOUNT" \
	'{
		enforcement: "enforced",
		allowedImagePrefixes: [$imagePrefix],
		allowedServiceAccounts: [$serviceAccount],
	allowedSecretRefs: []
	}')" | jq -e '.enforcement == "enforced" and (.allowedImagePrefixes | index("busybox:"))' >/dev/null

echo "Setting enforced project quota policy"
api_json PUT "/v1/projects/$project_id/quota-policy" "$(jq -n \
	'{
		enforcement: "enforced",
		maxActiveSandboxes: 5,
		maxRetainedArtifactBytes: 1048576
	}')" | jq -e '.enforcement == "enforced" and .maxActiveSandboxes == 5 and .maxRetainedArtifactBytes == 1048576' >/dev/null
cli_json projects quota-policy "$project_id" | jq -e '.enforcement == "enforced" and .maxActiveSandboxes == 5' >/dev/null

echo "Registering project credential reference"
credential_json="$(jq -n '{
	name: "Smoke Git credential",
	slug: "smoke-git",
	type: "git",
	target: "https://github.com/mlhiter/mbox",
	secretRef: {name: "smoke-git-token", key: "token"},
	usage: ["clone", "fetch"]
}')"
credential="$(api_json POST "/v1/projects/$project_id/credentials" "$credential_json")"
credential_id="$(jq -r '.id' <<<"$credential")"
api_json GET "/v1/projects/$project_id/credentials" | jq -e --arg credentialId "$credential_id" '.items[] | select(.id == $credentialId and .secretRef.name == "smoke-git-token")' >/dev/null
api_json GET "/v1/projects/$project_id/usage" | jq -e '.templates.projectScoped == 2 and .credentials.total == 1 and .sandboxes.total == 0' >/dev/null

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

echo "Checking runtime resource observation"
api_json GET "/v1/runtime/resources?namespace=$NAMESPACE&kind=SandboxClaim" | jq -e \
	--arg claim "$claim_name" \
	--arg pod "$pod_name" \
	'. as $inventory |
	($inventory.items[] | select(.name == $claim)) as $claimResource |
	$claimResource.observation.podName == $pod and
	$claimResource.observation.podPhase == "Running" and
	$claimResource.observation.podCount >= 1 and
	$claimResource.observation.runningPodCount >= 1 and
	$claimResource.observation.containersReady >= 1 and
	$claimResource.observation.requests.cpu == "50m" and
	$claimResource.observation.requests.memory == "64Mi" and
	$inventory.summary.workload.observedResources >= 1 and
	$inventory.summary.workload.observedPods >= 1 and
	$inventory.summary.workload.runningPods >= 1 and
	$inventory.summary.workload.requests.cpu == "50m" and
	$inventory.summary.workload.requests.memory == "64Mi"' >/dev/null
cli_json runtime resources --summary --namespace "$NAMESPACE" --kind SandboxClaim | jq -e '
	.total >= 1 and
	.workload.observedResources >= 1 and
	.workload.observedPods >= 1 and
	.workload.runningPods >= 1 and
	.workload.requests.cpu == "50m" and
	.workload.requests.memory == "64Mi"
' >/dev/null

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

echo "Checking workspace PVC contract"
template_runtime_name="$("${kubectl_cmd[@]}" get "sandboxclaim.extensions.agents.x-k8s.io/$claim_name" -n "$NAMESPACE" -o jsonpath='{.spec.sandboxTemplateRef.name}')"
template_mount_path="$("${kubectl_cmd[@]}" get "sandboxtemplate.extensions.agents.x-k8s.io/$template_runtime_name" -n "$NAMESPACE" -o jsonpath='{.spec.podTemplate.spec.containers[0].volumeMounts[?(@.name=="workspace")].mountPath}')"
template_storage_request="$("${kubectl_cmd[@]}" get "sandboxtemplate.extensions.agents.x-k8s.io/$template_runtime_name" -n "$NAMESPACE" -o jsonpath='{.spec.volumeClaimTemplates[?(@.metadata.name=="workspace")].spec.resources.requests.storage}')"
if [[ "$template_mount_path" != "/workspace" ]]; then
	echo "expected template workspace mount /workspace, got $template_mount_path" >&2
	exit 1
fi
if [[ "$template_storage_request" != "1Gi" ]]; then
	echo "expected template workspace storage request 1Gi, got $template_storage_request" >&2
	exit 1
fi
pvc_name="$("${kubectl_cmd[@]}" get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.spec.volumes[?(@.name=="workspace")].persistentVolumeClaim.claimName}')"
if [[ -z "$pvc_name" ]]; then
	echo "expected pod workspace volume to use a PersistentVolumeClaim" >&2
	exit 1
fi
"${kubectl_cmd[@]}" wait -n "$NAMESPACE" "pvc/$pvc_name" --for=jsonpath='{.status.phase}'=Bound --timeout="${TIMEOUT_SECONDS}s"

echo "Checking pod logs and workspace exec"
"${kubectl_cmd[@]}" logs -n "$NAMESPACE" "$pod_name" | grep -F "mbox sandbox ready" >/dev/null
"${kubectl_cmd[@]}" exec -n "$NAMESPACE" "$pod_name" -- sh -c 'echo smoke-ok > /workspace/mbox-smoke.txt && cat /workspace/mbox-smoke.txt' | grep -F "smoke-ok" >/dev/null

echo "Checking workspace data survives pod replacement"
pod_uid="$("${kubectl_cmd[@]}" get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.metadata.uid}')"
"${kubectl_cmd[@]}" delete pod "$pod_name" -n "$NAMESPACE" --wait=false >/dev/null
replacement_pod_name="$(wait_replacement_pod_name "$pod_selector" "$pod_uid")"
"${kubectl_cmd[@]}" wait -n "$NAMESPACE" "pod/$replacement_pod_name" --for=condition=Ready --timeout="${TIMEOUT_SECONDS}s"
replacement_pvc_name="$("${kubectl_cmd[@]}" get pod "$replacement_pod_name" -n "$NAMESPACE" -o jsonpath='{.spec.volumes[?(@.name=="workspace")].persistentVolumeClaim.claimName}')"
if [[ "$replacement_pvc_name" != "$pvc_name" ]]; then
	echo "expected replacement pod to reuse PVC $pvc_name, got $replacement_pvc_name" >&2
	exit 1
fi
"${kubectl_cmd[@]}" exec -n "$NAMESPACE" "$replacement_pod_name" -- sh -c 'cat /workspace/mbox-smoke.txt' | grep -F "smoke-ok" >/dev/null
pod_name="$replacement_pod_name"

echo "Declaring and checking preview port metadata"
api_json PATCH "/v1/sandboxes/$sandbox_id" "$(jq -n '{ports: [{name: "web", port: 8080, protocol: "TCP"}]}')" >/dev/null
wait_preview_port "$sandbox_id" 8080

echo "Waiting for mbox API sandbox status running"
wait_api_sandbox_status "$sandbox_id" running

echo "Checking mbox template validation APIs"
validation="$(api_json POST "/v1/templates/$template_id/validation-runs" "$(jq -n --arg projectId "$project_id" '{projectId: $projectId, metadata: {caller: "runtime-smoke"}}')")"
validation_sandbox_id="$(jq -r '.sandbox.id' <<<"$validation")"
jq -e --arg sandboxId "$validation_sandbox_id" '.template.metadata.validationStatus == "testing" and .template.metadata.validationSandboxId == $sandboxId and .sandbox.metadata.purpose == "environment-validation"' <<<"$validation" >/dev/null
api_json POST "/v1/templates/$template_id/validation-runs/$validation_sandbox_id/decision" "$(jq -n '{status: "passed"}')" | jq -e '.template.metadata.validationStatus == "passed" and .sandbox.metadata.validationResult == "passed"' >/dev/null
validation_claim_name="$(wait_claim_name "$validation_sandbox_id")"
api_json DELETE "/v1/sandboxes/$validation_sandbox_id" >/dev/null || true
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$validation_claim_name"
wait_api_sandbox_runtime_ref_cleared "$validation_sandbox_id"
validation_sandbox_id=""

echo "Checking mbox runtime access APIs"
api_json GET "/v1/sandboxes/$sandbox_id/runtime" | jq -e --arg pod "$pod_name" --arg pvc "$pvc_name" '.podName == $pod and .container != "" and (.storage[] | select(.mountPath == "/workspace" and .claimName == $pvc and .phase == "Bound"))' >/dev/null
api_json GET "/v1/sandboxes/$sandbox_id/boundary" | jq -e --arg namespace "$NAMESPACE" '.namespace == $namespace and .serviceAccountName == "mbox-sandbox" and .serviceAccountTokenAutomount == false and .policyEnforcement == "enforced" and .credentialProjection == "references-recorded-not-mounted" and (.credentialRefs[] | select(.slug == "smoke-git" and .secretRef == "smoke-git-token")) and (.checks[] | select(.id == "runtime-ref" and .status == "pass")) and (.checks[] | select(.id == "launch-policy" and .status == "pass")) and (.checks[] | select(.id == "credential-refs" and .status == "warn"))' >/dev/null
api_json GET "/v1/templates/$template_id/boundary?projectId=$project_id" | jq -e --arg namespace "$NAMESPACE" '.namespace == $namespace and .networkPolicyProjection == "agent-sandbox-managed-baseline" and .credentialProjection == "references-recorded-not-mounted" and .policyEnforcement == "enforced"' >/dev/null
api_json GET "/v1/templates/$lifecycle_template_id/boundary?projectId=$project_id" | jq -e '.lifecyclePolicyProjection == "ttl-enforced" and (.checks[] | select(.id == "lifecycle-policy" and .status == "pass"))' >/dev/null
api_json GET "/v1/sandboxes/$sandbox_id/logs?tailLines=20" | jq -e '.logs | contains("mbox sandbox ready")' >/dev/null
api_json GET "/v1/sandboxes/$sandbox_id/events" | jq -e '.items | type == "array"' >/dev/null

echo "Checking mbox lifecycle TTL cleanup"
lifecycle_sandbox_json="$(jq -n \
	--arg projectId "$project_id" \
	--arg templateId "$lifecycle_template_id" \
	--arg name "TTL $RUN_ID" \
	--arg slug "ttl-$RUN_ID" \
	'{projectId: $projectId, templateId: $templateId, name: $name, slug: $slug}')"
lifecycle_sandbox="$(api_json POST /v1/sandboxes "$lifecycle_sandbox_json")"
lifecycle_sandbox_id="$(jq -r '.id' <<<"$lifecycle_sandbox")"
lifecycle_claim_name="$(wait_claim_name "$lifecycle_sandbox_id")"
wait_api_sandbox_not_found "$lifecycle_sandbox_id"
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$lifecycle_claim_name"
wait_api_sandbox_runtime_ref_cleared "$lifecycle_sandbox_id"

echo "Checking mbox runtime session APIs"
session_json="$(jq -n '{
	type: "custom",
	client: "smoke-script",
	metadata: {purpose: "runtime-smoke"}
}')"
session="$(api_json POST "/v1/sandboxes/$sandbox_id/sessions" "$session_json")"
session_id="$(jq -r '.id' <<<"$session")"
api_json GET "/v1/sandboxes/$sandbox_id/sessions" | jq -e --arg sessionId "$session_id" '.items[] | select(.id == $sessionId and .status == "active" and .type == "custom")' >/dev/null
api_json POST "/v1/sessions/$session_id/end" | jq -e '.status == "ended" and (.endedAt | type == "string")' >/dev/null

echo "Checking mbox task watch and artifact content APIs"
task_json="$(jq -n '{
	command: ["sh", "-lc", "printf task-watch-ok | tee /workspace/task-watch.txt"],
	timeoutSeconds: 30
}')"
task="$(api_json POST "/v1/sandboxes/$sandbox_id/tasks" "$task_json")"
task_id="$(jq -r '.id' <<<"$task")"
task_status="$(wait_task_done "$task_id")"
if [[ "$task_status" != "succeeded" ]]; then
	echo "expected task $task_id to succeed, got $task_status" >&2
	exit 1
fi
api_raw GET "/v1/tasks/$task_id/events" | jq -s -e '
	(.[] | select(.type == "snapshot")) and
	(.[] | select(.type == "done" and .task.status == "succeeded" and (.task.stdout | contains("task-watch-ok"))))
' >/dev/null

artifact_json="$(jq -n \
	--arg taskId "$task_id" \
	'{
		taskId: $taskId,
		kind: "report",
		name: "task-watch.txt",
		uri: "workspace:///workspace/task-watch.txt",
		contentType: "text/plain"
	}')"
artifact="$(api_json POST "/v1/sandboxes/$sandbox_id/artifacts" "$artifact_json")"
artifact_id="$(jq -r '.id' <<<"$artifact")"
api_raw GET "/v1/artifacts/$artifact_id/content" | grep -F "task-watch-ok" >/dev/null
api_json POST "/v1/artifacts/$artifact_id/capture" | jq -e --arg backend "$EXPECTED_ARTIFACT_CONTENT_BACKEND" '.retainedContent.sizeBytes > 0 and (.retainedContent.sha256 | length == 64) and .retainedContent.storageProvider == $backend' >/dev/null

echo "Checking mbox CLI task, session, and artifact paths"
cli_session="$(cli_json sessions create "$sandbox_id" --type custom --client cli-runtime-smoke)"
cli_session_id="$(jq -r '.id' <<<"$cli_session")"
cli_json sessions end "$cli_session_id" | jq -e '.status == "ended"' >/dev/null
cli_task="$(cli_json tasks create "$sandbox_id" --arg sh --arg -lc --arg 'printf cli-task-ok | tee /workspace/cli-task.txt' --timeout 30)"
cli_task_id="$(jq -r '.id' <<<"$cli_task")"
cli_task_status="$(wait_task_done "$cli_task_id")"
if [[ "$cli_task_status" != "succeeded" ]]; then
	echo "expected CLI task $cli_task_id to succeed, got $cli_task_status" >&2
	exit 1
fi
cli_json tasks watch "$cli_task_id" | jq -s -e '(.[] | select(.type == "done" and .task.status == "succeeded" and (.task.stdout | contains("cli-task-ok"))))' >/dev/null
cli_artifact_json="$(jq -n \
	--arg taskId "$cli_task_id" \
	'{
		taskId: $taskId,
		kind: "report",
		name: "cli-task.txt",
		uri: "workspace:///workspace/cli-task.txt",
		contentType: "text/plain"
	}')"
cli_artifact="$(api_json POST "/v1/sandboxes/$sandbox_id/artifacts" "$cli_artifact_json")"
cli_artifact_id="$(jq -r '.id' <<<"$cli_artifact")"
cli_json artifacts capture "$cli_artifact_id" | jq -e --arg backend "$EXPECTED_ARTIFACT_CONTENT_BACKEND" '.retainedContent.sizeBytes > 0 and (.retainedContent.sha256 | length == 64) and .retainedContent.storageProvider == $backend' >/dev/null
cli_json artifacts content "$cli_artifact_id" | grep -F "cli-task-ok" >/dev/null

echo "Checking mbox client artifact upload path"
upload_artifact_json="$(jq -n \
	'{
		kind: "report",
		name: "client-upload.txt",
		uri: "client://smoke/client-upload.txt",
		contentType: "text/plain"
	}')"
upload_artifact="$(api_json POST "/v1/sandboxes/$sandbox_id/artifacts" "$upload_artifact_json")"
upload_artifact_id="$(jq -r '.id' <<<"$upload_artifact")"
printf 'client-upload-ok' | cli_json artifacts upload "$upload_artifact_id" --stdin --content-type text/plain | jq -e --arg backend "$EXPECTED_ARTIFACT_CONTENT_BACKEND" '.retainedContent.sizeBytes == 16 and (.retainedContent.sha256 | length == 64) and .retainedContent.storageProvider == $backend' >/dev/null
cli_json artifacts content "$upload_artifact_id" | grep -F "client-upload-ok" >/dev/null
api_json GET "/v1/projects/$project_id/usage" | jq -e '.sandboxes.active >= 1 and .runtimeSessions.total >= 2 and .executionTasks.total >= 2 and .artifacts.total >= 3 and .artifacts.retainedContent >= 3 and .artifacts.retainedBytes > 0' >/dev/null
cli_json projects usage "$project_id" | jq -e '.sandboxes.active >= 1 and .executionTasks.succeeded >= 2 and .artifacts.retainedBytes > 0' >/dev/null
cli_json projects audit-events "$project_id" --limit 20 | jq -e '
	(.items[] | select(.action == "runtime.session.created")) and
	(.items[] | select(.action == "execution.task.created")) and
	(.items[] | select(.action == "artifact.content.captured")) and
	(.items[] | select(.action == "artifact.content.uploaded"))
' >/dev/null

echo "Deleting sandbox through mbox API"
api_json DELETE "/v1/sandboxes/$sandbox_id" >/dev/null || true
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$claim_name"
wait_api_sandbox_runtime_ref_cleared "$sandbox_id"
api_json GET "/v1/projects/$project_id/usage" | jq -e '.sandboxes.deleted >= 1 and .sandboxes.cleanupPending == 0' >/dev/null
echo "Checking runtime orphan audit after cleanup"
api_json GET "/v1/runtime/orphans?namespace=$NAMESPACE" | jq -e --arg namespace "$NAMESPACE" '.adapter == "agent-sandbox" and .namespace == $namespace and .orphanCount == 0 and .expectedClean == true' >/dev/null
cli_json runtime orphans --namespace "$NAMESPACE" | jq -e --arg namespace "$NAMESPACE" '.adapter == "agent-sandbox" and .namespace == $namespace and .orphanCount == 0 and .expectedClean == true' >/dev/null

echo "Checking gated runtime orphan cleanup"
orphan_claim_name="manual-orphan-$RUN_ID"
orphan_sandbox_id="00000000-0000-4000-8000-000000000001"
"${kubectl_cmd[@]}" apply -n "$NAMESPACE" -f - >/dev/null <<EOF
apiVersion: extensions.agents.x-k8s.io/v1alpha1
kind: SandboxClaim
metadata:
  name: $orphan_claim_name
  labels:
    app.kubernetes.io/name: mbox
    app.kubernetes.io/managed-by: mbox
    mbox.dev/project-id: 00000000-0000-4000-8000-000000000002
    mbox.dev/sandbox-id: $orphan_sandbox_id
spec:
  sandboxTemplateRef:
    name: missing-template
EOF
api_json GET "/v1/runtime/orphans?namespace=$NAMESPACE" | jq -e \
	--arg name "$orphan_claim_name" \
	'.items[] | select(.reason == "missing-sandbox-record" and .resource.kind == "SandboxClaim" and .resource.name == $name)' >/dev/null
cleanup_payload="$(jq -n \
	--arg namespace "$NAMESPACE" \
	--arg name "$orphan_claim_name" \
	'{
		resource: {
			adapter: "agent-sandbox",
			kind: "SandboxClaim",
			namespace: $namespace,
			name: $name
		},
		reason: "missing-sandbox-record",
		deleteOrphan: true,
		confirm: "delete-orphan-runtime-resource"
	}')"
api_json POST "/v1/runtime/orphans/cleanup?namespace=$NAMESPACE" "$cleanup_payload" | jq -e --arg name "$orphan_claim_name" '.deleted == true and .resource.name == $name and .reason == "missing-sandbox-record"' >/dev/null
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$orphan_claim_name"
api_json GET "/v1/runtime/orphans?namespace=$NAMESPACE" | jq -e --arg namespace "$NAMESPACE" '.adapter == "agent-sandbox" and .namespace == $namespace and .orphanCount == 0 and .expectedClean == true' >/dev/null
cli_orphan_claim_name="manual-cli-orphan-$RUN_ID"
"${kubectl_cmd[@]}" apply -n "$NAMESPACE" -f - >/dev/null <<EOF
apiVersion: extensions.agents.x-k8s.io/v1alpha1
kind: SandboxClaim
metadata:
  name: $cli_orphan_claim_name
  labels:
    app.kubernetes.io/name: mbox
    app.kubernetes.io/managed-by: mbox
    mbox.dev/project-id: 00000000-0000-4000-8000-000000000003
    mbox.dev/sandbox-id: 00000000-0000-4000-8000-000000000004
spec:
  sandboxTemplateRef:
    name: missing-template
EOF
cli_json runtime cleanup-orphan \
	--adapter agent-sandbox \
	--kind SandboxClaim \
	--namespace "$NAMESPACE" \
	--name "$cli_orphan_claim_name" \
	--reason missing-sandbox-record \
	--confirm delete-orphan-runtime-resource | jq -e --arg name "$cli_orphan_claim_name" '.deleted == true and .resource.name == $name' >/dev/null
wait_deleted "sandboxclaim.extensions.agents.x-k8s.io/$cli_orphan_claim_name"
cli_json runtime orphans --namespace "$NAMESPACE" | jq -e --arg namespace "$NAMESPACE" '.adapter == "agent-sandbox" and .namespace == $namespace and .orphanCount == 0 and .expectedClean == true' >/dev/null
if "${kubectl_cmd[@]}" get "pvc/$pvc_name" -n "$NAMESPACE" >/dev/null 2>&1; then
	echo "Workspace PVC $pvc_name remains after SandboxClaim deletion; namespace cleanup will remove it"
else
	echo "Workspace PVC $pvc_name was deleted with the SandboxClaim"
fi

echo "Deleting smoke project through mbox API"
api_json DELETE "/v1/projects/$project_id" >/dev/null

echo "Smoke passed"
echo "project=$project_id template=$template_id sandbox=$sandbox_id namespace=$NAMESPACE claim=$claim_name pod=$pod_name"
