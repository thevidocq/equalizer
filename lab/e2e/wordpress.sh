#!/usr/bin/env bash
# Prepares the lab's legacy site: WordPress with SQLite (no MySQL), served by
# PHP's own server on 127.0.0.1:18090 as http://www.acme.test.
set -euo pipefail
cd "$(dirname "$0")"
RUN="$PWD/.run"
WP="$RUN/wordpress"
WP_TAG="${WP_TAG:-6.9.12}"
SQLITE_TAG="${SQLITE_TAG:-v3.1.3}"

if [ ! -f "$WP/wp-includes/version.php" ]; then
  git -c advice.detachedHead=false clone -q --depth 1 --branch "$WP_TAG" https://github.com/WordPress/WordPress.git "$WP"
fi
# The plugin lives in a monorepo: clone it apart and link its package.
src="$RUN/sqlite-src"
if [ ! -d "$src" ]; then
  git -c advice.detachedHead=false clone -q --depth 1 --branch "$SQLITE_TAG" https://github.com/WordPress/sqlite-database-integration.git "$src"
fi
plugin="$WP/wp-content/plugins/sqlite-database-integration"
[ -e "$plugin" ] || ln -s "$src/packages/plugin-sqlite-database-integration" "$plugin"
sed -e "s#{SQLITE_IMPLEMENTATION_FOLDER_PATH}#$plugin#" -e "s#{SQLITE_PLUGIN}#sqlite-database-integration/load.php#" \
  "$plugin/db.copy" > "$WP/wp-content/db.php"

if [ ! -f "$WP/wp-config.php" ]; then
  salts=$(python3 -c "import secrets;print(''.join(f\"define('{k}', '{secrets.token_hex(32)}');\n\" for k in ['AUTH_KEY','SECURE_AUTH_KEY','LOGGED_IN_KEY','NONCE_KEY','AUTH_SALT','SECURE_AUTH_SALT','LOGGED_IN_SALT','NONCE_SALT']))")
  cat > "$WP/wp-config.php" <<CONF
<?php
define('DB_NAME', 'wordpress'); define('DB_USER', ''); define('DB_PASSWORD', ''); define('DB_HOST', '');
define('DB_DIR', '$RUN/wp-db/'); define('DB_FILE', 'wordpress.sqlite');
define('WP_HOME', 'http://www.acme.test'); define('WP_SITEURL', 'http://www.acme.test');
define('AUTOMATIC_UPDATER_DISABLED', true); define('WP_HTTP_BLOCK_EXTERNAL', true);
$salts
\$table_prefix = 'wp_';
define('WP_DEBUG', false);
if (!defined('ABSPATH')) { define('ABSPATH', __DIR__ . '/'); }
require_once ABSPATH . 'wp-settings.php';
CONF
  mkdir -p "$RUN/wp-db"
fi
