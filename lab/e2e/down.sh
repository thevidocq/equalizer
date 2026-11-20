#!/usr/bin/env bash
# Stops the local lab. --clean also removes its data (databases, installed
# sites, mail, secrets): the next up.sh starts from nothing. The built
# sources in .run/src are kept.
set -uo pipefail
cd "$(dirname "$0")" || exit 1
RUN="$PWD/.run"
[ -f "$RUN/pids" ] && while read -r _ pid; do kill "$pid" 2>/dev/null; done < "$RUN/pids"
pkill -f "\.run/bin/" 2>/dev/null # also what was started by hand, by any path
pkill -f "php -S 127.0.0.1:18(09|10)" 2>/dev/null # the legacy sites
pkill -f "mariadbd --no-defaults --user=mysql --datadir=/var/tmp/equalizer-lab-mysql" 2>/dev/null
pkill -f "python3 -m equalizer_agent" 2>/dev/null # the discovery agent
PG="/var/tmp/equalizer-lab-pg"
PGBIN=$(find /usr/lib/postgresql -mindepth 2 -maxdepth 2 -name bin | sort -V | tail -1)
[ -d "$PG" ] && su postgres -c "$PGBIN/pg_ctl -D $PG stop -m fast" >/dev/null 2>&1
if [ "${1:-}" = "--clean" ]; then
  rm -rf "$PG" /var/tmp/equalizer-lab-mysql "$RUN/wp-db" "$RUN/wordpress/wp-config.php" "$RUN/joomla" "$RUN/drupal" "$RUN/intranet" "$RUN/intranet-db" "$RUN/cart" "$RUN/shop" "$RUN/forum" "$RUN/forum-db" "$RUN/wiki" "$RUN/wiki-db" "$RUN/site" "$RUN/store" "$RUN/mail" "$RUN/secrets" "$RUN/maps" "$RUN/logs"
  sed -i '/# equalizer-lab/d' /etc/hosts
fi
echo "Lab down."
