#!/usr/bin/env bash
# Backs up an Equalizer installation: the panel database (tenants, people,
# maps, connectors, settings), the secrets and .env. Logs (Loki) are not
# included: they expire after 30 days anyway.
#
#   ./backup.sh [destination directory]          default: ./backups
#
# EQ_BACKUP_PASSPHRASE set: the archive is encrypted (AES-256, openssl);
# without it the archive holds the secrets in clear: keep it safe.
# EQ_BACKUP_KEEP: how many archives to keep in the destination (default 14).
set -euo pipefail
cd "$(dirname "$0")"

dest="${1:-./backups}"
keep="${EQ_BACKUP_KEEP:-14}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
umask 077
mkdir -p "$dest"

echo "Dumping the database…"
docker compose exec -T postgres pg_dump -U equalizer -d equalizer -Fc > "$work/database.dump"
cp -a secrets "$work/secrets"
[ -f .env ] && cp .env "$work/env"
cat > "$work/README.txt" <<INFO
Equalizer backup $stamp
version: $(grep -s '^EQ_VERSION=' .env | cut -d= -f2 || true)
restore: ./restore.sh <this archive>
INFO

name="equalizer-backup-$stamp.tar.gz"
files=(database.dump secrets README.txt)
[ -f "$work/env" ] && files+=(env)
tar -C "$work" -czf "$work/$name" "${files[@]}"
if [ -n "${EQ_BACKUP_PASSPHRASE:-}" ]; then
  openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass env:EQ_BACKUP_PASSPHRASE \
    -in "$work/$name" -out "$dest/$name.enc"
  out="$dest/$name.enc"
else
  mv "$work/$name" "$dest/$name"
  out="$dest/$name"
  echo "Warning: the archive holds the secrets in clear (set EQ_BACKUP_PASSPHRASE to encrypt it)." >&2
fi
echo "Backup written: $out"

# Keep the newest $keep archives (names sort by date).
shopt -s nullglob
all=("$dest"/equalizer-backup-*.tar.gz*)
for ((i = 0; i < ${#all[@]} - keep; i++)); do
  rm -f -- "${all[i]}"
done
