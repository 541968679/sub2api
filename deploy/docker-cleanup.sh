#!/bin/sh
set -eu

BUILD_CACHE_MAX_AGE="${BUILD_CACHE_MAX_AGE:-24h}"
LOG_FILE="${LOG_FILE:-/opt/sub2api/docker-cleanup.log}"

log() {
    printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG_FILE"
}

log "=== Docker cleanup started ==="
docker builder prune -af --filter "until=${BUILD_CACHE_MAX_AGE}" >> "$LOG_FILE" 2>&1 || \
    log "WARN: docker builder prune failed"
docker image prune -f >> "$LOG_FILE" 2>&1 || \
    log "WARN: docker image prune failed"
docker system df >> "$LOG_FILE" 2>&1 || \
    log "WARN: docker system df failed"
df -h / >> "$LOG_FILE" 2>&1 || \
    log "WARN: df failed"
log "=== Docker cleanup finished ==="
