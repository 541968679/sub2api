#!/usr/bin/env bash
set -euo pipefail

COMPOSE_DIR="${COMPOSE_DIR:-/opt/sub2api}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-/opt/sub2api/backups}"
LOG_FILE="${LOG_FILE:-/opt/sub2api/backup.log}"
STAMP="$(date '+%Y%m%d-%H%M%S')"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG_FILE"
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

cd "$COMPOSE_DIR"
set -a
. "$ENV_FILE"
set +a
mkdir -p "$BACKUP_DIR"

log "Starting PostgreSQL logical backup"
compose exec -T postgres pg_dump -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" | gzip -9 > "$BACKUP_DIR/postgres-${STAMP}.sql.gz"

log "Archiving app data directory"
tar -czf "$BACKUP_DIR/data-${STAMP}.tar.gz" data

log "Triggering Redis background save"
compose exec -T redis redis-cli ${REDIS_PASSWORD:+-a "$REDIS_PASSWORD"} BGSAVE >/dev/null || log "WARN: Redis BGSAVE command failed"

log "Backup complete: $BACKUP_DIR"
