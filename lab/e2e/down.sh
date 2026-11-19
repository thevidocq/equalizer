#!/usr/bin/env bash
# Stops the local lab. --clean also removes its data (database, WordPress,
# mail, secrets): the next up.sh starts from nothing.
set -uo pipefail
cd "$(dirname "$0")" || exit 1
RUN="$PWD/.run"
[ -f "$RUN/pids" ] && while read -r _ pid; do kill "$pid" 2>/dev/null; done < "$RUN/pids"
pkill -f "$RUN/bin/" 2>/dev/null # also what was started by hand
pkill -f "php -S 127.0.0.1:18090" 2>/dev/null
PG="/var/tmp/equalizer-lab-pg"
PGBIN=$(find /usr/lib/postgresql -mindepth 2 -maxdepth 2 -name bin | sort -V | tail -1)
[ -d "$PG" ] && su postgres -c "$PGBIN/pg_ctl -D $PG stop -m fast" >/dev/null 2>&1
if [ "${1:-}" = "--clean" ]; then
  rm -rf "$PG" "$RUN/wp-db" "$RUN/wordpress/wp-config.php" "$RUN/mail" "$RUN/secrets" "$RUN/maps" "$RUN/logs"
  sed -i '/# equalizer-lab/d' /etc/hosts
fi
echo "Lab down."
