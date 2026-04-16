#!/bin/bash
# =============================================================================
# Sub2API Safe Deployment Script
# =============================================================================
# Features:
#   - Builds to a staging tag first, only retags on success
#   - Keeps the previous image for instant rollback
#   - Auto-rolls back if health check fails after deploy
#   - Logs everything to /opt/sub2api/deploy.log
#   - Safe against SSH disconnections (use with: nohup bash update.sh &)
#
# Usage:
#   bash /opt/sub2api/update.sh           # normal deploy
#   bash /opt/sub2api/update.sh --rollback # rollback to previous version
# =============================================================================

set -euo pipefail

REPO_DIR="/opt/sub2api/repo"
COMPOSE_DIR="/opt/sub2api"
IMAGE_NAME="sub2api-custom"
STAGING_TAG="${IMAGE_NAME}:staging"
LATEST_TAG="${IMAGE_NAME}:latest"
PREV_TAG="${IMAGE_NAME}:prev"
HEALTH_URL="http://localhost:8080/health"
HEALTH_RETRIES=5
HEALTH_INTERVAL=5
LOG_FILE="/opt/sub2api/deploy.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

health_check() {
    local retries=$1
    local interval=$2
    for i in $(seq 1 "$retries"); do
        if curl -sf -o /dev/null -m 5 "$HEALTH_URL"; then
            log "Health check passed (attempt $i/$retries)"
            return 0
        fi
        log "Health check attempt $i/$retries failed, waiting ${interval}s..."
        sleep "$interval"
    done
    return 1
}

do_rollback() {
    log "=== ROLLBACK: Restoring previous version ==="
    if ! docker image inspect "$PREV_TAG" >/dev/null 2>&1; then
        log "ERROR: No previous image ($PREV_TAG) found, cannot rollback!"
        return 1
    fi
    docker tag "$PREV_TAG" "$LATEST_TAG"
    cd "$COMPOSE_DIR"
    docker compose up -d
    log "Waiting for rollback health check..."
    sleep 8
    if health_check 3 5; then
        log "=== ROLLBACK SUCCESSFUL ==="
        docker compose ps
        return 0
    else
        log "=== ROLLBACK HEALTH CHECK FAILED — manual intervention required ==="
        return 1
    fi
}

do_deploy() {
    log "=============================================="
    log "=== Starting deployment ==="
    log "=============================================="

    # 1. Pull latest code
    log "--- Pulling latest code ---"
    cd "$REPO_DIR"
    git pull origin main 2>&1 | tee -a "$LOG_FILE"

    # 2. Build to staging tag (old image stays intact during build)
    #    --no-cache ensures code changes are always picked up (Docker's
    #    COPY layer cache can miss in-place git-pull updates).
    log "--- Building image to staging tag (no-cache) ---"
    if ! docker build --no-cache -t "$STAGING_TAG" . 2>&1 | tee -a "$LOG_FILE"; then
        log "ERROR: Build failed! Production is still running the old image, no downtime."
        return 1
    fi

    # 3. Preserve current latest as prev (for rollback)
    if docker image inspect "$LATEST_TAG" >/dev/null 2>&1; then
        log "--- Saving current image as prev ---"
        docker tag "$LATEST_TAG" "$PREV_TAG"
    fi

    # 4. Promote staging to latest
    log "--- Promoting staging to latest ---"
    docker tag "$STAGING_TAG" "$LATEST_TAG"

    # 5. Restart with new image
    log "--- Restarting sub2api ---"
    cd "$COMPOSE_DIR"
    docker compose up -d 2>&1 | tee -a "$LOG_FILE"

    # 6. Health check with auto-rollback
    log "--- Health check (${HEALTH_RETRIES} attempts, ${HEALTH_INTERVAL}s interval) ---"
    sleep 5
    if health_check "$HEALTH_RETRIES" "$HEALTH_INTERVAL"; then
        log "=== Deployment successful ==="
        docker compose ps
        # Clean up staging tag
        docker rmi "$STAGING_TAG" 2>/dev/null || true
    else
        log "ERROR: Health check failed after deploy!"
        do_rollback
        return 1
    fi

    log "=============================================="
    log "=== Done ==="
    log "=============================================="
}

# --- Main ---
case "${1:-}" in
    --rollback)
        do_rollback
        ;;
    *)
        do_deploy
        ;;
esac
