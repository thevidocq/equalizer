#!/usr/bin/env bash
# Restores an Equalizer backup made by backup.sh, on this installation or on
# a new VPS with the same release checked out.
#
#   ./restore.sh <archive>               .tar.gz, or .tar.gz.enc with EQ_BACKUP_PASSPHRASE
#
# The panel database is replaced by the backup's. The secrets and .env are
# put back only where missing (a new VPS); existing ones are kept, unless
# EQ_RESTORE_SECRETS=yes.
set -euo pipefail
cd "$(dirname "$0")"

archive="${1:?usage: ./restore.sh <archive>}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
umask 077

case "$archive" in
  *.enc)
    : "${EQ_BACKUP_PASSPHRASE:?the archive is encrypted: set EQ_BACKUP_PASSPHRASE}"
    openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass env:EQ_BACKUP_PASSPHRASE -in "$archive" -out "$work/backup.tar.gz"
    ;;
  *) cp "$archive" "$work/backup.tar.gz" ;;
esac
tar -C "$work" -xzf "$work/backup.tar.gz"
cat "$work/README.txt"

read -r -p "This replaces the panel database. Type RESTORE to go on: " answer
[ "$answer" = "RESTORE" ] || { echo "Nothing changed."; exit 1; }

if [ "${EQ_RESTORE_SECRETS:-}" = "yes" ] || [ ! -f secrets/map-signing.key ]; then
  echo "Putting back the secrets…"
  mkdir -p secrets
  cp -a "$work/secrets/." secrets/
fi
if [ -f "$work/env" ] && { [ "${EQ_RESTORE_SECRETS:-}" = "yes" ] || [ ! -f .env ]; }; then
  cp "$work/env" .env
fi

echo "Stopping the panel…"
docker compose stop panel-api >/dev/null
docker compose up -d --wait postgres >/dev/null
echo "Restoring the database…"
docker compose exec -T postgres dropdb -U equalizer --if-exists equalizer
docker compose exec -T postgres createdb -U equalizer equalizer
docker compose exec -T postgres pg_restore -U equalizer -d equalizer --no-owner < "$work/database.dump"
echo "Starting everything…"
docker compose up -d >/dev/null
echo "Restored. Gateways load the published maps from the panel within a minute."
