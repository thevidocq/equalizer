#!/usr/bin/env bash
# Prepares the lab's other web applications, each served by PHP's own server
# and used to prove the agent's profiles (agent/equalizer_agent/webapps):
#   OpenCart   on 127.0.0.1:18101 as http://cart.acme.test  (MariaDB)
#   PrestaShop on 127.0.0.1:18102 as http://shop.acme.test  (MariaDB)
#   phpBB      on 127.0.0.1:18103 as http://forum.acme.test (SQLite)
#   MediaWiki  on 127.0.0.1:18104 as http://wiki.acme.test  (SQLite)
#   TYPO3      on 127.0.0.1:18105 as http://site.acme.test  (MariaDB)
#   Magento    on 127.0.0.1:18106 as http://store.acme.test (MariaDB), with
#              its search modules off: OpenSearch is not reachable here
# Each has a test account "editor" / "Lab-editor-pass-1" where its visitors
# log in: a customer on the shops (by e-mail, editor@acme.test), the
# administrator on the forum and the wiki. TYPO3 has no visitor login.
# Sources come from GitHub only (no Packagist) and are kept in .run/src.
# MariaDB must be running on 127.0.0.1:33061 (up.sh starts it).
set -euo pipefail
cd "$(dirname "$0")"
LAB="$PWD"
RUN="$LAB/.run"
SRC="$RUN/src"
OPENCART_TAG="${OPENCART_TAG:-4.0.2.3}"
PRESTASHOP_TAG="${PRESTASHOP_TAG:-8.2.3}"
PHPBB_TAG="${PHPBB_TAG:-release-3.3.15}"
MEDIAWIKI_TAG="${MEDIAWIKI_TAG:-REL1_43}"
TYPO3_TAG="${TYPO3_TAG:-v13.4.40}"
MAGENTO_TAG="${MAGENTO_TAG:-2.4.7-p9}"
PASS="Lab-editor-pass-1"
DB=(mariadb -h 127.0.0.1 -P 33061 -u root)
export COMPOSER_ALLOW_SUPERUSER=1 COMPOSER_NO_INTERACTION=1
mkdir -p "$SRC" "$RUN/logs"

clone() { # repo, tag, dir
  [ -d "$3/.git" ] || GIT_LFS_SKIP_SMUDGE=1 git -c advice.detachedHead=false clone -q --depth 1 --branch "$2" "$1" "$3"
}
copy() { # from, to, excluded paths...
  local from="$1" to="$2"; shift 2
  local ex=(); for e in "$@"; do ex+=(--exclude="./$e"); done
  rm -rf "$to" && mkdir -p "$to"
  (cd "$from" && tar "${ex[@]}" -cf - .) | tar -xf - -C "$to"
}
fresh_db() { "${DB[@]}" -e "drop database if exists $1; create database $1 character set utf8mb4"; }
hash() { php -r 'echo password_hash($argv[1], PASSWORD_DEFAULT);' "$1"; }

# --- OpenCart -------------------------------------------------------------
O="$SRC/opencart-$OPENCART_TAG"
if [ ! -d "$O" ]; then
  curl -fsSL -o "$SRC/opencart.zip" "https://github.com/opencart/opencart/releases/download/$OPENCART_TAG/opencart-$OPENCART_TAG.zip"
  mkdir -p "$O.tmp" && unzip -q "$SRC/opencart.zip" -d "$O.tmp" && mv "$O.tmp/opencart-$OPENCART_TAG/upload" "$O"
  rm -rf "$O.tmp" "$SRC/opencart.zip"
fi
if [ ! -f "$RUN/cart/config.php" ] || ! "${DB[@]}" -e "use opencart" 2>/dev/null; then
  copy "$O" "$RUN/cart"
  cp "$RUN/cart/config-dist.php" "$RUN/cart/config.php" && cp "$RUN/cart/admin/config-dist.php" "$RUN/cart/admin/config.php"
  fresh_db opencart
  (cd "$RUN/cart" && php install/cli_install.php install --username editor --password "$PASS" --email editor@acme.test \
    --http_server http://cart.acme.test/ --db_driver mysqli --db_hostname 127.0.0.1 --db_username root --db_password "" \
    --db_database opencart --db_port 33061 --db_prefix oc_ > "$RUN/logs/opencart-install.log")
  rm -rf "$RUN/cart/install"
  "${DB[@]}" opencart -e "update oc_setting set value='Acme Cart' where \`key\` in ('config_name', 'config_meta_title');
    insert into oc_customer (customer_group_id, store_id, language_id, firstname, lastname, email, telephone, password,
      custom_field, newsletter, ip, status, safe, token, code, date_added)
    values (1, 0, 1, 'Lab', 'Editor', 'editor@acme.test', '', '$(hash "$PASS")', '', 0, '', 1, 0, '', '', now());"
fi

# --- PrestaShop -----------------------------------------------------------
P="$SRC/prestashop-$PRESTASHOP_TAG"
if [ ! -d "$P" ]; then
  curl -fsSL -o "$SRC/prestashop.zip" "https://github.com/PrestaShop/PrestaShop/releases/download/$PRESTASHOP_TAG/prestashop_$PRESTASHOP_TAG.zip"
  mkdir -p "$P.tmp" && unzip -q "$SRC/prestashop.zip" -d "$P.tmp" && unzip -q "$P.tmp/prestashop.zip" -d "$P"
  rm -rf "$P.tmp" "$SRC/prestashop.zip"
fi
if [ ! -f "$RUN/shop/app/config/parameters.php" ] || ! "${DB[@]}" -e "use prestashop" 2>/dev/null; then
  copy "$P" "$RUN/shop"
  # The installer downloads language packs: English is built in, stubs do.
  tar -czf "$RUN/shop/translations/en.gzip" -T /dev/null
  (cd "$RUN/shop/translations" && echo "lab" > README.lab && zip -q sf-en-US.zip README.lab && rm README.lab)
  fresh_db prestashop
  (cd "$RUN/shop" && php -d memory_limit=1G install/index_cli.php --domain=shop.acme.test --db_server=127.0.0.1:33061 \
    --db_name=prestashop --db_user=root --db_password= --prefix=ps_ --name="Acme Shop" --email=editor@acme.test \
    --password="$PASS" --firstname=Lab --lastname=Editor --language=en --country=us --ssl=0 --newsletter=0 \
    --send_email=0 > "$RUN/logs/prestashop-install.log" 2>&1)
  rm -rf "$RUN/shop/install"
  php "$LAB/prestashop-setup.php" "$RUN/shop" "$PASS"
fi

# --- phpBB ----------------------------------------------------------------
B="$SRC/phpbb"
clone https://github.com/phpbb/phpbb "$PHPBB_TAG" "$B"
[ -f "$B/phpBB/vendor/autoload.php" ] || (cd "$B/phpBB" && composer install -q --no-dev --prefer-source --no-progress)
if [ ! -f "$RUN/forum-db/forum.sqlite" ]; then
  copy "$B/phpBB" "$RUN/forum"
  rm -rf "$RUN/forum-db" && mkdir -p "$RUN/forum-db"
  sed -e "s#@DB@#$RUN/forum-db/forum.sqlite#" -e "s#@PASS@#$PASS#" "$LAB/phpbb-install.yml" > "$RUN/forum-db/install.yml"
  (cd "$RUN/forum" && php install/phpbbcli.php install "$RUN/forum-db/install.yml" > "$RUN/logs/phpbb-install.log")
  rm -rf "$RUN/forum/install" "$RUN/forum-db/install.yml" # the board stays closed while it is there
fi

# --- MediaWiki ------------------------------------------------------------
W="$SRC/mediawiki"
clone https://github.com/wikimedia/mediawiki "$MEDIAWIKI_TAG" "$W"
clone https://github.com/wikimedia/mediawiki-vendor "$MEDIAWIKI_TAG" "$W/vendor"
clone https://github.com/wikimedia/mediawiki-skins-Vector "$MEDIAWIKI_TAG" "$W/skins/Vector"
if [ ! -f "$RUN/wiki/LocalSettings.php" ]; then
  copy "$W" "$RUN/wiki" .git vendor/.git skins/Vector/.git tests
  rm -rf "$RUN/wiki-db" && mkdir -p "$RUN/wiki-db"
  (cd "$RUN/wiki" && php maintenance/run.php install --dbtype=sqlite --dbpath="$RUN/wiki-db" --dbname=wiki \
    --server="http://wiki.acme.test" --scriptpath="" --lang=en --pass="$PASS" --skins=Vector "Acme Wiki" "Editor" \
    > "$RUN/logs/mediawiki-install.log")
  for page in "Acme handbook" "Office hours"; do
    echo "The '''$page''' page of the Acme Wiki. See also [[Main Page]]." |
      (cd "$RUN/wiki" && php maintenance/run.php edit --user Editor --summary lab "$page" >> "$RUN/logs/mediawiki-install.log")
  done
fi

# --- TYPO3 ----------------------------------------------------------------
T="$SRC/typo3"
clone https://github.com/TYPO3/typo3 "$TYPO3_TAG" "$T"
[ -f "$T/vendor/autoload.php" ] && [ -f "$T/bin/typo3" ] || (cd "$T" && composer install -q --no-dev --prefer-source --no-progress)
if [ ! -f "$RUN/site/typo3conf/system/settings.php" ] || ! "${DB[@]}" -e "use typo3" 2>/dev/null; then
  copy "$T" "$RUN/site" .git Build
  fresh_db typo3
  "${DB[@]}" -e "create user if not exists 'lab'@'127.0.0.1' identified by 'Lab-db-pass-1';
    grant all on typo3.* to 'lab'@'127.0.0.1'"
  # TYPO3's own password policy wants a special character: the backend
  # account gets one more; the visitors' side has no login anyway.
  (cd "$RUN/site" && php bin/typo3 setup --driver=mysqli --host=127.0.0.1 --port=33061 --dbname=typo3 --username=lab \
    --password=Lab-db-pass-1 --admin-username=editor --admin-user-password="$PASS!" --admin-email=editor@acme.test \
    --project-name="Acme Site" --create-site="http://site.acme.test/" --server-type=other --force -n \
    > "$RUN/logs/typo3-install.log")
  php "$LAB/typo3-setup.php" "$RUN/site"
  "${DB[@]}" typo3 < "$LAB/typo3-content.sql"
  (cd "$RUN/site" && php bin/typo3 cache:flush -q)
fi

# --- Magento --------------------------------------------------------------
M="$SRC/magento"
clone https://github.com/magento/magento2 "$MAGENTO_TAG" "$M"
# Without plugins: one of them asks Packagist about every package.
[ -f "$M/vendor/autoload.php" ] || (cd "$M" && composer install -q --no-dev --no-plugins --prefer-source --no-progress)
if [ ! -f "$RUN/store/app/etc/env.php" ] || ! "${DB[@]}" -e "use magento" 2>/dev/null; then
  copy "$M" "$RUN/store" .git dev/tests
  fresh_db magento
  (cd "$RUN/store" && php -d memory_limit=4G bin/magento setup:install --base-url=http://store.acme.test/ \
    --db-host=127.0.0.1:33061 --db-name=magento --db-user=root --db-password= --admin-firstname=Lab \
    --admin-lastname=Editor --admin-email=editor@acme.test --admin-user=editor --admin-password="$PASS" \
    --language=en_US --currency=USD --timezone=UTC --use-rewrites=0 --backend-frontname=admin \
    --disable-modules=Magento_Elasticsearch,Magento_Elasticsearch7,Magento_OpenSearch,Magento_AdvancedSearch \
    > "$RUN/logs/magento-install.log" 2>&1)
  (cd "$RUN/store" && php bin/magento config:set -q system/smtp/disable 1)
  php -d memory_limit=2G "$LAB/magento-setup.php" "$RUN/store" "$PASS"
  (cd "$RUN/store" && php bin/magento cache:flush -q)
fi
