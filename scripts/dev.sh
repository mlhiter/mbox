#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"

POSTGRES_CONTAINER="${MBOX_POSTGRES_CONTAINER:-mbox-postgres}"
POSTGRES_IMAGE="${MBOX_POSTGRES_IMAGE:-postgres:17}"
POSTGRES_PORT="${MBOX_POSTGRES_PORT:-5432}"
POSTGRES_USER="${MBOX_POSTGRES_USER:-mbox}"
POSTGRES_PASSWORD="${MBOX_POSTGRES_PASSWORD:-mbox}"
POSTGRES_DB="${MBOX_POSTGRES_DB:-mbox}"
DATABASE_URL="${DATABASE_URL:-postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@127.0.0.1:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable}"
API_ADDR="${MBOX_LISTEN_ADDR:-127.0.0.1:18080}"
API_URL="http://$API_ADDR"
WEB_PORT="${MBOX_WEB_PORT:-5174}"
ENABLE_RUNTIME="${MBOX_DEV_RUNTIME:-false}"
USE_DOCKER="${MBOX_DEV_DOCKER_POSTGRES:-true}"

api_pid=""
web_pid=""

usage() {
	cat <<'EOF'
Usage: scripts/dev.sh [options]

Starts the local mbox stack:
  - Postgres in Docker unless disabled
  - Go API server on 127.0.0.1:18080 by default
  - Vite web console on 127.0.0.1:5174 by default

Options:
  --runtime     Enable Kubernetes runtime controller and runtime access.
  --no-docker   Do not start Docker Postgres; use DATABASE_URL as provided.
  -h, --help    Show this help.

Useful environment variables:
  DATABASE_URL
  MBOX_LISTEN_ADDR
  MBOX_WEB_PORT
  MBOX_KUBECONFIG
  MBOX_KUBE_CONTEXT
  MBOX_POSTGRES_PORT
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--runtime)
			ENABLE_RUNTIME="true"
			shift
			;;
		--no-docker)
			USE_DOCKER="false"
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			echo "unknown option: $1" >&2
			usage >&2
			exit 1
			;;
	esac
done

cleanup() {
	if [[ -n "$web_pid" ]] && kill -0 "$web_pid" >/dev/null 2>&1; then
		kill "$web_pid" >/dev/null 2>&1 || true
	fi
	if [[ -n "$api_pid" ]] && kill -0 "$api_pid" >/dev/null 2>&1; then
		kill "$api_pid" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT INT TERM

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 1
	fi
}

wait_http() {
	local url="$1"
	local label="$2"
	local deadline=$((SECONDS + 60))
	while ((SECONDS < deadline)); do
		if curl -fsS "$url" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "timed out waiting for $label at $url" >&2
	exit 1
}

start_postgres() {
	if [[ "$USE_DOCKER" != "true" ]]; then
		echo "Skipping Docker Postgres; using DATABASE_URL=$DATABASE_URL"
		return
	fi

	require_command docker

	if docker ps --format '{{.Names}}' | grep -Fx "$POSTGRES_CONTAINER" >/dev/null; then
		echo "Postgres container $POSTGRES_CONTAINER is already running"
	elif docker ps -a --format '{{.Names}}' | grep -Fx "$POSTGRES_CONTAINER" >/dev/null; then
		echo "Starting existing Postgres container $POSTGRES_CONTAINER"
		docker start "$POSTGRES_CONTAINER" >/dev/null
	else
		echo "Creating Postgres container $POSTGRES_CONTAINER on port $POSTGRES_PORT"
		docker run --name "$POSTGRES_CONTAINER" \
			-e POSTGRES_USER="$POSTGRES_USER" \
			-e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
			-e POSTGRES_DB="$POSTGRES_DB" \
			-p "$POSTGRES_PORT:5432" \
			-d "$POSTGRES_IMAGE" >/dev/null
	fi
}

start_api() {
	require_command go
	echo "Starting API server at $API_URL"
	(
		cd "$ROOT_DIR"
		export DATABASE_URL
		export MBOX_LISTEN_ADDR="$API_ADDR"
		if [[ "$ENABLE_RUNTIME" == "true" ]]; then
			export MBOX_RUNTIME_CONTROLLER_ENABLED=true
			export MBOX_RUNTIME_ACCESS_ENABLED=true
			export MBOX_KUBECONFIG="${MBOX_KUBECONFIG:-$HOME/.kube/config}"
			export MBOX_KUBE_CONTEXT="${MBOX_KUBE_CONTEXT:-kind-agent-sandbox}"
			echo "Runtime enabled with kube context ${MBOX_KUBE_CONTEXT}"
		fi
		go run ./cmd/mbox-server
	) &
	api_pid="$!"
	wait_http "$API_URL/healthz" "API server"
}

start_web() {
	require_command npm
	if [[ ! -d "$WEB_DIR/node_modules" ]]; then
		echo "Installing web dependencies"
		(cd "$WEB_DIR" && npm install)
	fi

	echo "Starting web console at http://127.0.0.1:$WEB_PORT"
	(
		cd "$WEB_DIR"
		export MBOX_API_PROXY_TARGET="$API_URL"
		export MBOX_WEB_PORT="$WEB_PORT"
		npm run dev
	) &
	web_pid="$!"
	wait_http "http://127.0.0.1:$WEB_PORT" "web console"
}

require_command curl
start_postgres
start_api
start_web

cat <<EOF

mbox dev stack is running.
  API: $API_URL
  Web: http://127.0.0.1:$WEB_PORT
  DB:  $DATABASE_URL

Press Ctrl-C to stop API and Web. The Postgres container is left running.
EOF

wait
