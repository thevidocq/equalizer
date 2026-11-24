#!/usr/bin/env bash
# Starts panel-api on an empty database with throwaway keys, creates the
# platform administrator and fills the panel with the demo organisation.
#   EQ_DATABASE_URL=postgres://… scripts/screenshots/stack.sh <work dir>
# Leaves panel-api running on :8080 (its pid in <work dir>/panel-api.pid).
set -euo pipefail
work=${1:?work dir}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
mkdir -p "$work"
key() { python3 -c 'import base64,os;print(base64.b64encode(os.urandom(32)).decode())'; }
export EQ_MAP_SIGNING_KEY=$(key) EQ_MFA_KEY=$(key)
export EQ_INTERNAL_TOKEN=$(python3 -c 'import secrets;print(secrets.token_urlsafe(32))')
export EQ_MFA_REQUIRED=false EQ_MAIL_MODE=log EQ_ADDR=127.0.0.1:8080 EQ_ADMIN_ADDR=127.0.0.1:9092 EQ_PANEL_URL=http://localhost:4173
export EQ_EXPOSURE_INTERVAL=0 EQ_VERSION=$(cat "$root/VERSION")
(cd "$root/panel-api" && go build -ldflags "-X main.version=$EQ_VERSION" -o "$work/panel-api" ./cmd/panel-api)
printf 'demo-password-2026\n' | "$work/panel-api" create-admin -email root@acme.example -name "Sam Rivera"
"$work/panel-api" > "$work/panel-api.log" 2>&1 &
echo $! > "$work/panel-api.pid"
for _ in $(seq 60); do curl -fs http://127.0.0.1:9092/healthz >/dev/null && break; sleep 0.5; done
python3 "$here/seed.py" http://127.0.0.1:8080 "$EQ_INTERNAL_TOKEN" "$work/panel-api.log" > "$work/accounts.json"
echo "panel-api ready, demo data in place"
