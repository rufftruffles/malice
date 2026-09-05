#!/bin/sh
# malice-update: nightly refresh of signature-decaying engine images.
#
# Rebuilds the engines whose value decays between builds (AV signatures and
# rule sets are fetched at build time) and, when MALICE_REGISTRY is set and
# docker is logged in, pushes them so client deployments pick them up with a
# plain `docker pull`.
#
# Engines refreshed:
#   clamav  - freshclam signatures (fetched at build time)
#   kvrt    - KAV definitions (fetched at build time)
#   yara    - rule set (fetched at build time)
#   lmd     - maldet signature DB (fetched at build time)
#
# NOT refreshed here:
#   eset    - signatures are refreshed PER SCAN by the entrypoint (upd -u),
#             so the image only needs rebuilding when the EEA version itself
#             changes (manual, version-driven).
#   others  - static tools or live-API engines (no decaying state).
#
# Environment:
#   MALICE_ENGINES_DIR  directory containing the engine repos (default: the
#                       parent of the malice repo this script ships in)
#   MALICE_REGISTRY     registry prefix to push to (e.g. ghcr.io/rufftruffles);
#                       empty = local rebuild only
#   MALICE_VERSION      public image version tag (default 1.0.0)
#
# Exit code: 0 if every engine built, 1 if any failed.
set -u

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ENGINES_DIR=${MALICE_ENGINES_DIR:-$(cd "$SCRIPT_DIR/../.." && pwd)}
REGISTRY=${MALICE_REGISTRY:-}
VERSION=${MALICE_VERSION:-1.0.0}

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }

cd "$ENGINES_DIR" || exit 1
fail=0

for e in clamav kvrt yara; do
    log "building $e"
    if (cd "$e" && make build && make tag); then
        log "built $e"
    else
        log "ERROR: build failed: $e"
        fail=1
    fi
done

log "building lmd"
if (cd lmd && make build && make tag); then
    log "built lmd"
else
    log "ERROR: build failed: lmd"
    fail=1
fi

if [ -n "$REGISTRY" ]; then
    for e in clamav kvrt yara lmd; do
        for t in "$VERSION" latest; do
            docker tag "malice/$e:latest" "$REGISTRY/malice-$e:$t" || { log "ERROR: tag failed: $e:$t"; fail=1; }
            docker push "$REGISTRY/malice-$e:$t" || { log "ERROR: push failed: $e:$t"; fail=1; }
        done
    done
    log "pushed refreshed images to $REGISTRY (v$VERSION)"
fi

# Rebuilds leave the previous image dangling; drop it.
docker image prune -f >/dev/null 2>&1 || true

log "done (fail=$fail)"
exit "$fail"
