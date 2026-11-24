# source-reader

Reads a site's code where the site lives and sends the panel a **summary**
of it (`schema/source/summary.schema.json`): entry points (the routes each
application declares, PHP files that run when opened), the type the code
gives each request key, the calls where a request value may do harm,
components and versions. Names, types and paths only: never code,
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
  - `local`: the site's folder mounted read only at its own path (or the
    volumes of its container), with `EQ_SOURCE_SITE=/`. A reader installed
    before 0.2.30 (no `EQ_SOURCE_SITE`, the folder at `/site`) still reads
    `/site`.
- **Never read**: the files with credentials of every application
  Equalizer supports, by name (`fetch.SecretFiles`: `wp-config.php`,
  `configuration.php`, `settings*.php`, `env.php`, `parameters.php`,
  `config.php`, `LocalSettings.php`, `LocalConfiguration.php`…), the
  usual names of a custom site's database settings, `.env`, keys and
  certificates; a file that holds a password written in the code is
  dropped unread (`fetch.SecretContent`); uploads, caches, version
  control, `vendor`, `node_modules`. Over sftp rsync excludes them; over
  smb the folder is listed first and only wanted files are copied. Only
  code (`.php`, `.phtml`, `.inc`, `.module`) and the files that name entry
  points or components (`fetch.codeNames`: `style.css`, `composer.json`,
  `composer.lock`, routing files, manifests), up to 1 MiB each. Copied: at
  most 150 000 files, 1 GiB (`MaxCopyFiles`, `MaxCopyBytes`); studied: 20 000
  files, 200 MiB of the site's own code (the core is never read). Files
  fetched over sftp or smb are read in a temporary folder deleted after
  the read.
- Patterns, not a PHP parser: what it cannot read it leaves out.

Environment: `EQ_PANEL_URL`, `EQ_SOURCE_TOKEN`, `EQ_SOURCE_STATE`
(`/var/lib/equalizer-source`), `EQ_SOURCE_WORK` (`/tmp/equalizer-source`),
`EQ_SOURCE_SITE` (`/site`, `/` with the command the panel gives), `EQ_LOG_LEVEL`.

Tests: `go test ./...`; `go test ./internal/extract -update` rewrites
`schema/source/examples/wordpress.json` from the fixture site, which
`scripts/validate_schemas.py` checks against the schema. `cms_test.go`
builds a small site of each application and checks what is read.

## What counts

- **Each application is a kind** (`stacks.go`, `cms_*.go`, `wordpress.go`):
  the files that find it (at the root or in one folder: `blog/`, `web/`,
  `public/`), its core (known code: only its version files are read; the
  rest is counted in `files.known` and never opened), who owns a file
  (`where.owner`: `plugin akismet`, `module webform`), what its files
  declare (REST routes, routing files, components, controllers, services)
  and which PHP files a browser opens directly (no guard such as
  `ABSPATH`, `_JEXEC`, `_PS_VERSION_`, `TYPO3`, `MEDIAWIKI`, `IN_PHPBB`,
  `MOODLE_INTERNAL`). WordPress, Drupal (8+ and 7), Joomla, Magento 2,
  PrestaShop, OpenCart, TYPO3, MediaWiki, phpBB, Moodle. A folder named
  `web`, `public`, `public_html`, `docroot`, `htdocs`, `html`, `www` or
  `upload` is the web server's root: its name is not in the paths, files
  outside it are not entry points.
- **Libraries** (`vendor/`, `third-party/`, `bower_components/`…) give only
  their versions (`composer.lock`).
- **Entry points** of a custom site are PHP files that act at their top
  level, outside named functions and classes: they read the request (in a
  double-quoted string or heredoc too) or make a risky call (`acts.go`). A
  closure is not a scope: handed to a hook it runs on every request.
  Read from braces, strings, heredocs, comments and attributes, not with a
  PHP parser: an approximation.
- **A backtick** is a command only around a request value on the same
  line; in a query it quotes a name.
