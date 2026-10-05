#!/usr/bin/env bash
set -euo pipefail

COMPOSE_DIR="${COMPOSE_DIR:-/opt/sub2api}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
ENV_FILE="${ENV_FILE:-.env}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/health}"
LOG_FILE="${LOG_FILE:-/opt/sub2api/deploy.log}"
HEALTH_RETRIES="${HEALTH_RETRIES:-12}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG_FILE"
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

health_check() {
  local i
  for i in $(seq 1 "$HEALTH_RETRIES"); do
    if curl -fsS -m 5 "$HEALTH_URL" >/dev/null; then
      log "Health check passed (${i}/${HEALTH_RETRIES})"
      return 0
    fi
    log "Health check failed (${i}/${HEALTH_RETRIES}); waiting ${HEALTH_INTERVAL}s"
    sleep "$HEALTH_INTERVAL"
  done
  return 1
}

image_ref() {
  compose config | awk '
    $1 == "sub2api:" { in_service=1; next }
    in_service && /^[[:space:]]{2}[A-Za-z0-9_-]+:/ { exit }
    in_service && $1 == "image:" { print $2; exit }
  '
}

env_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1); exit }' "$ENV_FILE"
}

verify_config() {
  cd "$COMPOSE_DIR"
  test -f "$COMPOSE_FILE" || { log "ERROR: missing $COMPOSE_DIR/$COMPOSE_FILE"; return 1; }
  test -f "$ENV_FILE" || { log "ERROR: missing $COMPOSE_DIR/$ENV_FILE"; return 1; }

  local image
  image="$(image_ref)"
  if [ -z "$image" ]; then
    log "ERROR: unable to resolve sub2api image from compose config"
    return 1
  fi
  log "Resolved sub2api image: $image"
}

require_linux_amd64() {
  local image="$1"
  local platform
  platform="$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')"
  if [ "$platform" != "linux/amd64" ]; then
    log "ERROR: $image platform is ${platform:-unknown}; production requires linux/amd64"
    return 1
  fi
  log "Image platform: $platform"
}

app_health_urls() {
  local bind port app_url
  bind="$(env_value BIND_HOST)"
  port="$(env_value SERVER_PORT)"
  bind="${bind:-127.0.0.1}"
  port="${port:-8080}"
  app_url="http://${bind}:${port}/health"
  printf '%s\n' "$app_url"
  if [ "$app_url" != "http://127.0.0.1:8080/health" ]; then
    printf '%s\n' "http://127.0.0.1:8080/health"
  fi
}

wait_for_health() {
  local url
  while IFS= read -r url; do
    [ -n "$url" ] || continue
    log "Checking $url"
    HEALTH_URL="$url" health_check || return 1
  done
}

report_status() {
  log "Container status"
  compose ps | tee -a "$LOG_FILE"
  docker inspect sub2api --format 'image={{.Config.Image}} status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}no-health{{end}}' | tee -a "$LOG_FILE"
}

deploy_app() {
  cd "$COMPOSE_DIR"
  verify_config
  local image
  image="$(image_ref)"

  log "Pulling sub2api image"
  compose pull sub2api
  require_linux_amd64 "$image"

  log "Recreating sub2api without postgres or redis"
  compose up -d --no-deps sub2api

  log "Waiting for health"
  if ! wait_for_health < <(app_health_urls); then
    log "ERROR: sub2api did not pass health check"
    report_status
    compose logs --tail=160 sub2api | tee -a "$LOG_FILE"
    return 1
  fi

  report_status
  log "Deployment complete"
}

deploy_stack() {
  cd "$COMPOSE_DIR"
  verify_config

  log "Pulling images"
  compose pull sub2api postgres redis

  log "Starting services"
  compose up -d

  log "Waiting for health"
  if ! wait_for_health < <(app_health_urls); then
    log "ERROR: sub2api did not pass health check"
    report_status
    compose logs --tail=160 sub2api | tee -a "$LOG_FILE"
    return 1
  fi

  report_status
  log "Stack deployment complete"
}

case "${1:-deploy}" in
  deploy|deploy-app)
    deploy_app
    ;;
  deploy-stack)
    deploy_stack
    ;;
  ps)
    cd "$COMPOSE_DIR"
    compose ps
    ;;
  logs)
    cd "$COMPOSE_DIR"
    compose logs --tail="${2:-160}" "${3:-sub2api}"
    ;;
  health)
    cd "$COMPOSE_DIR"
    wait_for_health < <(app_health_urls)
    ;;
  *)
    echo "Usage: $0 [deploy|deploy-stack|ps|logs [tail] [service]|health]" >&2
    exit 2
    ;;
esac
