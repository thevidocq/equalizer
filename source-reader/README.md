# source-reader

Reads a site's code where the site lives and sends the panel a **summary**
of it (`schema/source/summary.schema.json`): entry points (WordPress REST
routes, AJAX and admin-post actions, PHP files that run when opened), the
type the code gives each request key, the calls where a request value may
do harm, components and versions. Names, types and paths only: never code,
never values, never secrets.

- **Next to the connector**, never on the web server: it only calls the
  panel (`EQ_PANEL_URL`, with the site's token `EQ_SOURCE_TOKEN`, from the
  site's Source tab), no inbound port.
- **Read only**, three ways, chosen in the panel:
  - `sftp`: rsync over ssh with the reader's own ed25519 key, made at the
    first start in `/var/lib/equalizer-source`; the panel shows its public
    half, to add to a read-only user's `authorized_keys`. The server's key
    is learnt the first time and checked after.
  - `smb`: smbclient with a read-only account; the password goes in a
    file only the reader reads, never on a command line.
  - `local`: the site's folder mounted read only at `/site`.
- **Never read**: the files with credentials of every application
  Equalizer supports, by name (`fetch.SecretFiles`: `wp-config.php`,
  `configuration.php`, `settings*.php`, `env.php`, `parameters.php`,
  `config.php`, `LocalSettings.php`, `LocalConfiguration.php`…), the
  usual names of a custom site's database settings, `.env`, keys and
  certificates; a file that holds a password written in the code is
  dropped unread (`fetch.SecretContent`); uploads, caches, version
  control, `node_modules`. Over sftp rsync excludes them; over smb the
  folder is listed first and only wanted files are copied. Only `.php`, `.phtml`, `.inc`, `style.css` and
  `composer.lock`, up to 1 MiB each, 20 000 files, 200 MiB. Files fetched
  over sftp or smb are read in a temporary folder deleted after the read.
- Patterns, not a PHP parser: what it cannot read it leaves out.

Environment: `EQ_PANEL_URL`, `EQ_SOURCE_TOKEN`, `EQ_SOURCE_STATE`
(`/var/lib/equalizer-source`), `EQ_SOURCE_WORK` (`/tmp/equalizer-source`),
`EQ_SOURCE_SITE` (`/site`), `EQ_LOG_LEVEL`.

Tests: `go test ./...`; `go test ./internal/extract -update` rewrites
`schema/source/examples/wordpress.json` from the fixture site, which
`scripts/validate_schemas.py` checks against the schema.
