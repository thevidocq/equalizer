#!/usr/bin/env bash
# Checks an installation after an update (or at any time): services up,
# migrations applied, no errors since the services started, connector on
# the expected release and connected.
#
#   ./check.sh [connector container name]        default: eq-connector
#
# Errors in the first seconds after a restart ("panel unreachable" while
# panel-api starts) are normal: gateway and relay retry on their own. Only
# errors after the "loaded" lines matter.
set -uo pipefail
cd "$(dirname "$0")" || exit 1

connector="${1:-eq-connector}"
want="$(sed -n 's/^EQ_VERSION=//p' .env 2>/dev/null)"
problems=0
say() { printf '\n== %s\n' "$1"; }
bad() { echo "PROBLEM: $1"; problems=$((problems + 1)); }

say "Services"
docker compose ps --format 'table {{.Service}}\t{{.Status}}'
docker compose ps --format '{{.Service}} {{.Status}}' | grep -viE ' up ' | grep . && bad "a service is not up"
docker compose ps --format '{{.Service}} {{.Status}}' | grep -iE 'unhealthy|restarting' && bad "a service is unhealthy or restarting"

say "Release (EQ_VERSION=${want:-unset})"
versions="$(docker compose logs panel-api gateway tunnel-relay 2>/dev/null |
  grep -E '"msg":"[a-z-]+ started"' | grep -oE '"version":"[^"]*"' | sort -u)"
echo "${versions:-no start line found}"
[ -n "$want" ] && [ -n "$versions" ] && [ "$versions" != "\"version\":\"$want\"" ] &&
  bad "services run another release than $want"

say "Migrations"
docker compose logs panel-api 2>/dev/null | grep -E '"migrations applied"|migration' | tail -3

say "Errors in the last 10 minutes (gateway, relay, panel-api, agent)"
errors="$(docker compose logs --since=10m gateway tunnel-relay panel-api agent 2>/dev/null |
  grep -iE '"level":"error"|panic|fatal' | grep -v 'panel unreachable' | tail -20)"
if [ -n "$errors" ]; then echo "$errors"; bad "errors in the logs"; else echo "none"; fi
docker compose logs --since=2m gateway tunnel-relay 2>/dev/null | grep -q 'panel unreachable' &&
  bad "gateway or relay still cannot reach panel-api"

say "Connector ($connector)"
if docker ps --format '{{.Names}}' | grep -qx "$connector"; then
  docker ps --filter "name=^${connector}$" --format '{{.Image}}  {{.Status}}'
  image="$(docker inspect "$connector" --format '{{.Config.Image}}')"
  [ -n "$want" ] && [ "${image##*:}" != "$want" ] && bad "connector image is ${image##*:}, expected $want"
  docker logs --tail=50 "$connector" 2>&1 | grep -q '"tunnel open"' || bad "connector tunnel not open"
  docker logs --tail=5 "$connector" 2>&1
else
  echo "not on this server (check it on its own server, or in the panel: Connectors)"
fi

say "Result"
if [ "$problems" -eq 0 ]; then echo "OK"; else echo "$problems problem(s) above"; exit 1; fi
