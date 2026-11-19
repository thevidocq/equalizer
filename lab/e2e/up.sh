#!/usr/bin/env bash
# Starts the local end-to-end lab on this machine, without Docker: the real
# Equalizer services built from this checkout, a real WordPress as the legacy
# site, and stand-ins for what only exists on a VPS (HTTPS front instead of
# Caddy, SMTP sink, identity provider). Everything listens on 127.0.0.1.
# Needs: Go, Node, PHP (pdo_sqlite, pdo_pgsql, gd, intl), Composer, PostgreSQL 16, Redis, Python 3, root
# (ports 80/443 and /etc/hosts).
set -euo pipefail
cd "$(dirname "$0")"
LAB="$PWD"
ROOT="$(cd ../.. && pwd)"
RUN="$LAB/.run"
BIN="$RUN/bin"
LOGS="$RUN/logs"
VERSION="${EQ_LAB_VERSION:-0.1.0}"
# Lab names never go through an HTTP proxy set in the environment.
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}127.0.0.1,localhost,.lab.test,.acme.test,acme.test"
export no_proxy="$NO_PROXY"
mkdir -p "$BIN" "$LOGS" "$RUN/mail" "$RUN/secrets" "$RUN/maps"
: > "$RUN/pids"

start() { # name, command...
  local name="$1"; shift
  nohup "$@" > "$LOGS/$name.log" 2>&1 &
  echo "$name $!" >> "$RUN/pids"
}
wait_http() { # url
  for _ in $(seq 1 60); do curl -ks -o /dev/null "$1" && return 0; sleep 0.5; done
  echo "not answering: $1" >&2; exit 1
}

echo "Building…"
for svc in gateway tunnel-relay connector panel-api; do
  main=$(find "$ROOT/$svc/cmd" -mindepth 1 -maxdepth 1 -type d | head -1)
  (cd "$ROOT/$svc" && go build -ldflags "-X main.version=$VERSION" -o "$BIN/$svc" "./cmd/$(basename "$main")")
done
(cd "$ROOT/tools" && go build -o "$BIN/eqtool" ./cmd/eqtool)
(cd "$LAB" && go build -o "$BIN/front" ./cmd/front && go build -o "$BIN/idp" ./cmd/idp)
(cd "$ROOT/panel-ui" && npm run build > "$LOGS/panel-ui-build.log" 2>&1)
python3 -c "import aiosmtpd" 2>/dev/null || pip install -q --break-system-packages aiosmtpd

echo "Secrets…"
S="$RUN/secrets"
[ -f "$S/map-signing.key" ] || "$BIN/eqtool" keygen -out "$S/map-signing.key"
[ -f "$S/form.key" ] || "$BIN/eqtool" formkey -out "$S/form.key"
[ -f "$S/mfa.key" ] || "$BIN/eqtool" formkey -out "$S/mfa.key"
for t in relay.token panel-internal.token; do [ -f "$S/$t" ] || "$BIN/eqtool" secret -out "$S/$t"; done

echo "Names…"
hosts="127.0.0.1 panel.lab.test relay.lab.test idp.lab.test www.acme.test acme.test old.acme.test news.acme.test docs.acme.test intranet.acme.test"
sed -i '/# equalizer-lab/d' /etc/hosts && echo "$hosts # equalizer-lab" >> /etc/hosts

echo "PostgreSQL and Redis…"
# The postgres user may not reach this checkout: its data lives in /var/tmp.
PG="/var/tmp/equalizer-lab-pg"
PGBIN=$(find /usr/lib/postgresql -mindepth 2 -maxdepth 2 -name bin | sort -V | tail -1)
if [ ! -d "$PG" ]; then
  mkdir -p "$PG" && chown postgres "$PG"
  su postgres -c "$PGBIN/initdb -D $PG -A trust -U postgres" > /dev/null
fi
chown -R postgres "$PG"
su postgres -c "$PGBIN/pg_ctl -D $PG -l $PG/postgres.log -o '-p 55441 -k /tmp -c listen_addresses=127.0.0.1' start" > /dev/null
for _ in $(seq 1 30); do psql -h 127.0.0.1 -p 55441 -U postgres -c 'select 1' >/dev/null 2>&1 && break; sleep 0.5; done
psql -h 127.0.0.1 -p 55441 -U postgres -tc "select 1 from pg_database where datname='equalizer'" | grep -q 1 ||
  psql -q -h 127.0.0.1 -p 55441 -U postgres -c "create database equalizer"
start redis redis-server --port 16379 --bind 127.0.0.1 --save "" --appendonly no

echo "Front, mail, identity provider, WordPress…"
start front "$BIN/front" -dir "$RUN" -panel-ui "$ROOT/panel-ui/dist"
start smtp python3 "$LAB/smtp_sink.py" "$RUN/mail"
start idp "$BIN/idp"
wait_http https://idp.lab.test/.well-known/openid-configuration
cat /etc/ssl/certs/ca-certificates.crt "$RUN/ca.pem" > "$RUN/ca-bundle.pem"
"$LAB/wordpress.sh"
start wordpress env PHP_CLI_SERVER_WORKERS=4 php -S 127.0.0.1:18090 -t "$RUN/wordpress" "$LAB/router.php"
wait_http http://127.0.0.1:18090/
if [ ! -f "$RUN/wp-db/wordpress.sqlite" ] || ! curl -s -H "Host: www.acme.test" http://127.0.0.1:18090/ | grep -q "Acme Blog"; then
  curl -s -o /dev/null -H "Host: www.acme.test" -X POST "http://127.0.0.1:18090/wp-admin/install.php?step=2" \
    --data-urlencode "weblog_title=Acme Blog" --data-urlencode "user_name=editor" \
    --data-urlencode "admin_password=Lab-editor-pass-1" --data-urlencode "admin_password2=Lab-editor-pass-1" \
    --data-urlencode "pw_weak=1" --data-urlencode "admin_email=editor@acme.test" --data-urlencode "blog_public=0"
fi
php "$LAB/wp-setup.php" "$RUN/wordpress" > /dev/null
"$LAB/cms.sh"
start joomla env PHP_CLI_SERVER_WORKERS=4 php -S 127.0.0.1:18091 -t "$RUN/joomla" "$LAB/router.php"
start drupal env -C "$RUN/drupal" PHP_CLI_SERVER_WORKERS=4 php -S 127.0.0.1:18092 -t "$RUN/drupal" "$RUN/drupal/.ht.router.php" # Drupal expects its root as working directory
start intranet env PHP_CLI_SERVER_WORKERS=4 INTRANET_DB="$RUN/intranet-db/intranet.sqlite" php -S 127.0.0.1:18093 -t "$RUN/intranet"
for port in 18091 18092 18093; do wait_http "http://127.0.0.1:$port/"; done

echo "Equalizer…"
export EQ_LOG_LEVEL=info
PANEL_ENV=(EQ_ADDR=127.0.0.1:18085 EQ_ADMIN_ADDR=127.0.0.1:19092
  "EQ_DATABASE_URL=postgres://postgres@127.0.0.1:55441/equalizer?sslmode=disable"
  "EQ_MAP_SIGNING_KEY_FILE=$S/map-signing.key" EQ_MAP_KEY_ID=k1 "EQ_INTERNAL_TOKEN_FILE=$S/panel-internal.token"
  "EQ_MFA_KEY_FILE=$S/mfa.key" EQ_PANEL_URL=https://panel.lab.test EQ_MAIL_MODE=smtp EQ_SMTP_ADDR=127.0.0.1:2525
  EQ_MAIL_FROM=equalizer@lab.test EQ_EXPOSURE_INTERVAL=0 EQ_RELAY_ADMIN=127.0.0.1:19091 EQ_GATEWAY_ADMIN=127.0.0.1:19090)
start panel-api env "${PANEL_ENV[@]}" "$BIN/panel-api"
wait_http http://127.0.0.1:19092/healthz
# The first platform administrator, created on the server as on a VPS.
if ! psql -h 127.0.0.1 -p 55441 -U postgres -d equalizer -tc "select 1 from users where email='root@lab.test'" | grep -q 1; then
  echo "Lab-root-password-1" | env "${PANEL_ENV[@]}" "$BIN/panel-api" create-admin -email root@lab.test -name "Lab Root" > /dev/null
fi
start relay env EQ_TUNNEL_ADDR=127.0.0.1:18081 EQ_INTERNAL_ADDR=127.0.0.1:18082 EQ_ADMIN_ADDR=127.0.0.1:19091 \
  EQ_CONNECTORS_URL=http://127.0.0.1:18085/v1/internal/connectors EQ_PANEL_TOKEN_FILE="$S/panel-internal.token" \
  EQ_GATEWAY_TOKEN_FILE="$S/relay.token" "$BIN/tunnel-relay"
start gateway env EQ_PUBLIC_ADDR=127.0.0.1:18080 EQ_ADMIN_ADDR=127.0.0.1:19090 \
  EQ_MAP_URL=http://127.0.0.1:18085/v1/internal/maps EQ_MAP_TOKEN_FILE="$S/panel-internal.token" \
  EQ_MAP_FILE="$RUN/maps/current.json" EQ_MAP_PUBKEY_FILE="$S/map-signing.pub" EQ_MAP_KEY_ID=k1 \
  EQ_FORM_KEY_FILE="$S/form.key" EQ_RELAY_URL=http://127.0.0.1:18082 EQ_RELAY_TOKEN_FILE="$S/relay.token" \
  EQ_REDIS_URL=redis://127.0.0.1:16379/0 EQ_TRUSTED_PROXIES=127.0.0.1/32 SSL_CERT_FILE="$RUN/ca-bundle.pem" \
  "$BIN/gateway"
wait_http http://127.0.0.1:19090/healthz
wait_http https://panel.lab.test/
echo "Lab up: https://panel.lab.test (mail in $RUN/mail, logs in $LOGS)"
