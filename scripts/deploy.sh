#!/usr/bin/env bash
set -Eeuo pipefail

APP_DIR="${APP_DIR:-/srv/simple_ai}"
STARTUP_WAIT_SECONDS="${STARTUP_WAIT_SECONDS:-8}"
DEPLOY_NETWORK="${DEPLOY_NETWORK:-ai-net}"
AUTO_CREATE_NETWORK="${AUTO_CREATE_NETWORK:-false}"
NEW_TAG="${NEW_TAG:-$(date +%Y%m%d%H%M%S)}"
DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}"
DOCKER_BUILD_PULL="${DOCKER_BUILD_PULL:-false}"

# Backend settings
BACKEND_DOCKERFILE="${BACKEND_DOCKERFILE:-$APP_DIR/Dockerfile}"
BACKEND_CONTEXT_DIR="${BACKEND_CONTEXT_DIR:-$APP_DIR}"
BACKEND_IMAGE_NAME="${BACKEND_IMAGE_NAME:-simple_ai_backend}"
BACKEND_CONTAINER_NAME="${BACKEND_CONTAINER_NAME:-backend}"
BACKEND_HOST_PORT="${BACKEND_HOST_PORT:-9090}"
BACKEND_CONTAINER_PORT="${BACKEND_CONTAINER_PORT:-9090}"
BACKEND_CONFIG_FILE="${BACKEND_CONFIG_FILE:-$APP_DIR/config/config.toml}"
BACKEND_HEALTHCHECK_URL="${BACKEND_HEALTHCHECK_URL:-}"

# Frontend settings
FRONTEND_DOCKERFILE="${FRONTEND_DOCKERFILE:-$APP_DIR/vue-frontend/Dockerfile}"
FRONTEND_CONTEXT_DIR="${FRONTEND_CONTEXT_DIR:-$APP_DIR/vue-frontend}"
FRONTEND_IMAGE_NAME="${FRONTEND_IMAGE_NAME:-simple_ai_frontend}"
FRONTEND_CONTAINER_NAME="${FRONTEND_CONTAINER_NAME:-frontend}"
FRONTEND_HOST_PORT="${FRONTEND_HOST_PORT:-80}"
FRONTEND_CONTAINER_PORT="${FRONTEND_CONTAINER_PORT:-80}"
FRONTEND_API_UPSTREAM="${FRONTEND_API_UPSTREAM:-$BACKEND_CONTAINER_NAME:$BACKEND_CONTAINER_PORT}"
FRONTEND_HEALTHCHECK_URL="${FRONTEND_HEALTHCHECK_URL:-}"

OLD_BACKEND_CONTAINER=""
OLD_FRONTEND_CONTAINER=""
BACKEND_ROLLBACK_AVAILABLE=0
FRONTEND_ROLLBACK_AVAILABLE=0

container_exists() {
  local name="$1"
  docker ps -a --format '{{.Names}}' | grep -Fxq "$name"
}

ensure_network() {
  if docker network inspect "$DEPLOY_NETWORK" >/dev/null 2>&1; then
    echo "[deploy] using docker network: $DEPLOY_NETWORK"
    return 0
  fi

  if [ "$AUTO_CREATE_NETWORK" = "true" ]; then
    echo "[deploy] docker network not found, creating: $DEPLOY_NETWORK"
    docker network create "$DEPLOY_NETWORK" >/dev/null
    return 0
  fi

  echo "[deploy] docker network not found: $DEPLOY_NETWORK"
  echo "[deploy] create it first or set AUTO_CREATE_NETWORK=true"
  exit 1
}

backup_container() {
  local current="$1"
  local old_ref_var="$2"
  local flag_var="$3"
  local backup_name="${current}_predeploy_$(date +%Y%m%d%H%M%S)"

  if container_exists "$current"; then
    echo "[deploy] backup container: $current -> $backup_name"
    if docker rename "$current" "$backup_name"; then
      docker stop "$backup_name" >/dev/null 2>&1 || true
      printf -v "$old_ref_var" '%s' "$backup_name"
      printf -v "$flag_var" '%s' "1"
    else
      echo "[deploy] failed to rename $current; removing stale container and continuing without rollback backup"
      docker rm -f "$current" >/dev/null
      printf -v "$old_ref_var" '%s' ""
      printf -v "$flag_var" '%s' "0"
    fi
  fi
}

restore_container() {
  local current="$1"
  local backup="$2"
  local available="$3"

  if [ "$available" -eq 1 ] && [ -n "$backup" ] && container_exists "$backup"; then
    docker rename "$backup" "$current" >/dev/null 2>&1 || true
    docker start "$current" >/dev/null 2>&1 || true
    echo "[rollback] restored container: $current"
  else
    echo "[rollback] no backup container for: $current"
  fi
}

check_running() {
  local name="$1"
  if [ "$(docker inspect --format '{{.State.Running}}' "$name")" != "true" ]; then
    echo "[deploy] container is not running: $name"
    false
  fi
}

check_url() {
  local url="$1"
  local label="$2"
  if [ -z "$url" ]; then
    return 0
  fi
  if ! command -v curl >/dev/null 2>&1; then
    echo "[deploy] curl not found, skip $label healthcheck"
    return 0
  fi
  echo "[deploy] checking $label health url: $url"
  curl -fsS --max-time 8 "$url" >/dev/null
}

rollback() {
  set +e
  echo "[rollback] start rollback"

  if container_exists "$FRONTEND_CONTAINER_NAME"; then
    docker rm -f "$FRONTEND_CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if container_exists "$BACKEND_CONTAINER_NAME"; then
    docker rm -f "$BACKEND_CONTAINER_NAME" >/dev/null 2>&1 || true
  fi

  restore_container "$BACKEND_CONTAINER_NAME" "$OLD_BACKEND_CONTAINER" "$BACKEND_ROLLBACK_AVAILABLE"
  restore_container "$FRONTEND_CONTAINER_NAME" "$OLD_FRONTEND_CONTAINER" "$FRONTEND_ROLLBACK_AVAILABLE"
}

on_err() {
  local code=$?
  local line=$1
  trap - ERR
  echo "[deploy] failed at line $line, exit code: $code"
  rollback
  exit "$code"
}

trap 'on_err $LINENO' ERR

echo "[deploy] app dir: $APP_DIR"
cd "$APP_DIR"
export DOCKER_BUILDKIT

BUILD_PULL_FLAG=""
if [ "$DOCKER_BUILD_PULL" = "true" ]; then
  BUILD_PULL_FLAG="--pull"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "[deploy] docker is not installed"
  exit 1
fi

if [ ! -f "$BACKEND_CONFIG_FILE" ]; then
  echo "[deploy] missing backend config file: $BACKEND_CONFIG_FILE"
  echo "[deploy] create it first, for example: cp config/config.toml.example config/config.toml"
  exit 1
fi

ensure_network

echo "[deploy] build backend image: $BACKEND_IMAGE_NAME:$NEW_TAG"
docker build ${BUILD_PULL_FLAG} \
  -f "$BACKEND_DOCKERFILE" \
  -t "$BACKEND_IMAGE_NAME:$NEW_TAG" \
  -t "$BACKEND_IMAGE_NAME:latest" \
  "$BACKEND_CONTEXT_DIR"

echo "[deploy] build frontend image: $FRONTEND_IMAGE_NAME:$NEW_TAG"
docker build ${BUILD_PULL_FLAG} \
  -f "$FRONTEND_DOCKERFILE" \
  -t "$FRONTEND_IMAGE_NAME:$NEW_TAG" \
  -t "$FRONTEND_IMAGE_NAME:latest" \
  "$FRONTEND_CONTEXT_DIR"

backup_container "$BACKEND_CONTAINER_NAME" OLD_BACKEND_CONTAINER BACKEND_ROLLBACK_AVAILABLE
backup_container "$FRONTEND_CONTAINER_NAME" OLD_FRONTEND_CONTAINER FRONTEND_ROLLBACK_AVAILABLE

echo "[deploy] start backend container: $BACKEND_CONTAINER_NAME"
docker run -d \
  --name "$BACKEND_CONTAINER_NAME" \
  --restart unless-stopped \
  --network "$DEPLOY_NETWORK" \
  -p "${BACKEND_HOST_PORT}:${BACKEND_CONTAINER_PORT}" \
  -v "$BACKEND_CONFIG_FILE:/app/config/config.toml:ro" \
  "$BACKEND_IMAGE_NAME:$NEW_TAG" >/dev/null

echo "[deploy] start frontend container: $FRONTEND_CONTAINER_NAME"
docker run -d \
  --name "$FRONTEND_CONTAINER_NAME" \
  --restart unless-stopped \
  --network "$DEPLOY_NETWORK" \
  -p "${FRONTEND_HOST_PORT}:${FRONTEND_CONTAINER_PORT}" \
  -e "API_UPSTREAM=$FRONTEND_API_UPSTREAM" \
  "$FRONTEND_IMAGE_NAME:$NEW_TAG" >/dev/null

sleep "$STARTUP_WAIT_SECONDS"
check_running "$BACKEND_CONTAINER_NAME"
check_running "$FRONTEND_CONTAINER_NAME"
check_url "$BACKEND_HEALTHCHECK_URL" "backend"
check_url "$FRONTEND_HEALTHCHECK_URL" "frontend"

if [ "$BACKEND_ROLLBACK_AVAILABLE" -eq 1 ] && [ -n "$OLD_BACKEND_CONTAINER" ] && container_exists "$OLD_BACKEND_CONTAINER"; then
  docker rm "$OLD_BACKEND_CONTAINER" >/dev/null
fi
if [ "$FRONTEND_ROLLBACK_AVAILABLE" -eq 1 ] && [ -n "$OLD_FRONTEND_CONTAINER" ] && container_exists "$OLD_FRONTEND_CONTAINER"; then
  docker rm "$OLD_FRONTEND_CONTAINER" >/dev/null
fi

trap - ERR

echo "[deploy] deploy finished"
docker ps --filter "name=^/${BACKEND_CONTAINER_NAME}$|^/${FRONTEND_CONTAINER_NAME}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
