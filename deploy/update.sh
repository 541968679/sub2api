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
#   - Also updates AIClient2API sidecar by pulling its CI-built image
#
# Usage:
#   bash /opt/sub2api/update.sh              # normal deploy (both sub2api and aiclient2api if present)
#   bash /opt/sub2api/update.sh --rollback   # rollback sub2api to previous version
#   bash /opt/sub2api/update.sh --skip-a2    # deploy sub2api only, skip aiclient2api
#   bash /opt/sub2api/update.sh --only-a2    # deploy aiclient2api only
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
DOCKER_BUILD_CACHE_MAX_AGE="${DOCKER_BUILD_CACHE_MAX_AGE:-24h}"

# AIClient2API sidecar image. Built by AIClient2API GitHub Actions and pulled here.
A2_IMAGE_NAME="${AICLIENT2API_IMAGE:-ghcr.io/541968679/aiclient2api:latest}"

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

cleanup_docker_disk() {
    log "--- Docker disk cleanup (build cache older than ${DOCKER_BUILD_CACHE_MAX_AGE}) ---"
    docker builder prune -af --filter "until=${DOCKER_BUILD_CACHE_MAX_AGE}" 2>&1 | tee -a "$LOG_FILE" || \
        log "WARN: docker builder prune failed"

    log "--- Docker image cleanup (dangling images only) ---"
    docker image prune -f 2>&1 | tee -a "$LOG_FILE" || \
        log "WARN: docker image prune failed"

    log "--- Docker disk usage after cleanup ---"
    docker system df 2>&1 | tee -a "$LOG_FILE" || \
        log "WARN: docker system df failed"
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
        cleanup_docker_disk
    else
        log "ERROR: Health check failed after deploy!"
        do_rollback
        return 1
    fi

    log "=============================================="
    log "=== Done ==="
    log "=============================================="
}

do_deploy_a2() {
    log "=============================================="
    log "=== Starting AIClient2API deployment ==="
    log "=============================================="

    cd "$COMPOSE_DIR"

    log "--- Pulling AIClient2API image: $A2_IMAGE_NAME ---"
    if ! docker compose pull aiclient2api 2>&1 | tee -a "$LOG_FILE"; then
        log "ERROR: AIClient2API image pull failed! Sidecar unchanged, sub2api unaffected."
        return 1
    fi

    log "--- Restarting aiclient2api container ---"
    docker compose up -d aiclient2api 2>&1 | tee -a "$LOG_FILE"

    sleep 5
    if docker compose ps aiclient2api | grep -q "Up"; then
        log "=== AIClient2API deployment successful ==="
    else
        log "ERROR: AIClient2API container did not reach Up state. Check: docker compose logs aiclient2api"
        return 1
    fi

    log "=============================================="
    log "=== AIClient2API Done ==="
    log "=============================================="
}

# --- Main ---
case "${1:-}" in
    --rollback)
        do_rollback
        ;;
    --only-a2)
        do_deploy_a2
        ;;
    --skip-a2)
        do_deploy
        ;;
    *)
        do_deploy
        do_deploy_a2 || log "WARN: AIClient2API deploy failed but sub2api is running"
        ;;
esac
