#!/usr/bin/env bash
# Prepares the lab's other legacy sites, each served by PHP's own server:
#   Joomla on 127.0.0.1:18091 as http://news.acme.test (PostgreSQL)
#   Drupal on 127.0.0.1:18092 as http://docs.acme.test (SQLite)
#   Acme Intranet, a custom old-style PHP app, on 127.0.0.1:18093 as
#   http://intranet.acme.test (SQLite, ISO-8859-1 pages)
# Sources come from GitHub only (no Packagist): dependencies are installed
# from each project's composer.lock with --prefer-source. The build is kept
# in .run/src; installed copies are made again after down.sh --clean.
set -euo pipefail
cd "$(dirname "$0")"
LAB="$PWD"
RUN="$LAB/.run"
SRC="$RUN/src"
JOOMLA_TAG="${JOOMLA_TAG:-5.4.9}"
DRUPAL_TAG="${DRUPAL_TAG:-11.4.13}"
PSQL=(psql -q -h 127.0.0.1 -p 55441 -U postgres)
export COMPOSER_ALLOW_SUPERUSER=1 COMPOSER_NO_INTERACTION=1
mkdir -p "$SRC"

clone() { # repo, tag, dir
  [ -d "$3/.git" ] || GIT_LFS_SKIP_SMUDGE=1 git -c advice.detachedHead=false clone -q --depth 1 --branch "$2" "$1" "$3"
}
copy() { # from, to, excluded paths...
  local from="$1" to="$2"; shift 2
  local ex=(); for e in "$@"; do ex+=(--exclude="./$e"); done
  rm -rf "$to" && mkdir -p "$to"
  (cd "$from" && tar "${ex[@]}" -cf - .) | tar -xf - -C "$to"
}

# --- Joomla ---------------------------------------------------------------
J="$SRC/joomla"
clone https://github.com/joomla/joomla-cms "$JOOMLA_TAG" "$J"
if [ ! -f "$J/libraries/vendor/autoload.php" ]; then
  # The lock points one package to a private security mirror: the same
  # commit is in the public repository.
  sed -i 's#git@github.com:joomla-framework/security-filter.git#https://github.com/joomla-framework/filter.git#' "$J/composer.lock"
  (cd "$J" && composer install -q --no-dev --prefer-source --no-progress --ignore-platform-req=ext-ldap)
fi
[ -d "$J/media/system/js" ] || (cd "$J" && npm ci --no-audit --no-fund > "$RUN/logs/joomla-build.log" 2>&1)
if [ ! -f "$RUN/joomla/configuration.php" ]; then
  copy "$J" "$RUN/joomla" node_modules .git build/media_source tests
  "${PSQL[@]}" -c "drop database if exists joomla" -c "create database joomla"
  (cd "$RUN/joomla" && php installation/joomla.php install -n -q --site-name="Acme News" --admin-user="Lab Editor" \
    --admin-username=editor --admin-password="Lab-editor-pass-1" --admin-email=editor@acme.test --db-type=pgsql \
    --db-host=127.0.0.1:55441 --db-user=postgres --db-pass="" --db-name=joomla --db-prefix=jos_)
  # Pretty URLs, as with Apache's rewrite rules, and the site's address:
  # under PHP's own server Joomla gets its root wrong on some admin pages.
  # shellcheck disable=SC2016 # PHP, not shell
  sed -i -e 's/public $sef_rewrite = false;/public $sef_rewrite = true;/' \
    -e "s#public \$live_site = '';#public \$live_site = 'http://news.acme.test';#" "$RUN/joomla/configuration.php"
  php "$LAB/joomla-setup.php" "$RUN/joomla"
fi

# --- Drupal ---------------------------------------------------------------
D="$SRC/drupal"
clone https://github.com/drupal/drupal "$DRUPAL_TAG" "$D"
[ -f "$D/vendor/autoload.php" ] || (cd "$D" && composer install -q --no-dev --prefer-source --no-progress)
if [ ! -f "$RUN/drupal/sites/default/settings.php" ]; then
  copy "$D" "$RUN/drupal" .git core/tests core/node_modules
  dr=(php -d error_reporting=24575 core/scripts/dr) # without the deprecation notices
  (cd "$RUN/drupal" && "${dr[@]}" install minimal --site-name="Acme Docs" --password="Lab-editor-pass-1" > "$RUN/logs/drupal-install.log" 2>&1)
  # The standard recipe stops halfway in this version: its parts, one by one.
  for r in core_recommended_front_end_theme core_recommended_admin_theme basic_html_format_editor \
    page_content_type article_content_type; do
    (cd "$RUN/drupal" && "${dr[@]}" recipe "core/recipes/$r" >> "$RUN/logs/drupal-install.log" 2>&1)
  done
  # The database path is relative to the working directory: make it absolute.
  chmod u+w "$RUN/drupal/sites/default/settings.php"
  sed -i "s#'database' => 'sites/default/files/.sqlite'#'database' => '$RUN/drupal/sites/default/files/.sqlite'#" \
    "$RUN/drupal/sites/default/settings.php"
  php -d error_reporting=24575 "$LAB/drupal-setup.php" "$RUN/drupal" >> "$RUN/logs/drupal-install.log" 2>&1
fi

# --- Acme Intranet --------------------------------------------------------
copy "$LAB/apps/intranet" "$RUN/intranet"
mkdir -p "$RUN/intranet/uploads" "$RUN/intranet-db"
