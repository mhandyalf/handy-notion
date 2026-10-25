#!/bin/bash
# Host-local deployment. Never sources application .env files or prints secrets.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
exec 9>/run/lock/handy-notion-update.lock
flock -w 900 9 || { echo 'Deployment lock timed out.' >&2; exit 1; }
target=${1:-latest}
if [[ $# -gt 1 || ( $target != latest && ! $target =~ ^[0-9a-f]{40}$ ) ]]; then
  echo 'Expected a full commit SHA or latest.' >&2
  exit 1
fi
cd /myproject/handy-notion
compose=(docker compose -f /myproject/handy-notion/docker-compose.prod.yml)
api=mhandyalf/handy-notion-api:$target
web=mhandyalf/handy-notion-web:$target
old_api=$(docker inspect -f '{{.Image}}' handynotion-api)
old_web=$(docker inspect -f '{{.Image}}' handynotion-web)
docker pull "$api"
docker pull "$web"
new_api=$(docker image inspect -f '{{.Id}}' "$api")
new_web=$(docker image inspect -f '{{.Id}}' "$web")
revision() { docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$1"; }
api_revision=$(revision "$new_api")
web_revision=$(revision "$new_web")
if [[ ! $api_revision =~ ^[0-9a-f]{40}$ || $api_revision != "$web_revision" || ( $target != latest && $api_revision != "$target" ) ]]; then
  if [[ $target != latest ]]; then
    echo 'Requested commit does not match both image revisions.' >&2
    exit 1
  fi
  echo 'Skipping update: latest images lack matching commit revisions.'
  exit 0
fi
healthy() {
  local i
  for i in {1..30}; do
    if curl --fail --silent --max-time 5 http://127.0.0.1:8082/health >/dev/null &&
       curl --fail --silent --max-time 5 http://127.0.0.1:8083/ >/dev/null; then
      return 0
    fi
    sleep 2
  done
  return 1
}
if [[ $new_api == "$old_api" && $new_web == "$old_web" ]]; then
  healthy || { echo 'Current containers failed health checks.' >&2; exit 1; }
  echo "Already current: $api_revision"
  exit 0
fi
umask 077
backup_dir=/var/backups/handy-notion
mkdir -p "$backup_dir"
backup="$backup_dir/postgres-$(date -u +%Y%m%dT%H%M%SZ)-$api_revision.sql.gz"
# Back up the shared cluster without displaying credentials. Abort if dump fails.
docker exec postgres sh -c 'exec pg_dumpall -U "${POSTGRES_USER:-postgres}"' | gzip > "$backup.partial"
gzip -t "$backup.partial"
test "$(gzip -dc "$backup.partial" | wc -c)" -gt 100
mv "$backup.partial" "$backup"
override=$(mktemp /run/handy-notion-images.XXXXXX.yml)
trap 'rm -f "$override"' EXIT
pin_images() { printf 'services:\n  api:\n    image: "%s"\n  web:\n    image: "%s"\n' "$1" "$2" > "$override"; }
pin_images "$new_api" "$new_web"
if "${compose[@]}" -f "$override" up -d --no-deps --pull never api web && healthy; then
  echo "Updated Handy Notes to $api_revision; backup: $backup"
else
  echo 'Update failed. Attempting previous images; database is NOT automatically restored.' >&2
  pin_images "$old_api" "$old_web"
  "${compose[@]}" -f "$override" up -d --no-deps --pull never api web
  healthy || echo 'CRITICAL: previous images also failed health checks; manual intervention required.' >&2
  exit 1
fi