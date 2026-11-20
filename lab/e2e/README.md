# Local end-to-end lab

A whole Equalizer platform on one machine, without Docker, to test what a
customer does before a manual test on a real server. The services are the
real ones, built from this checkout; the legacy site is a real WordPress.
What only exists on a VPS is replaced by small stand-ins:

| On a VPS | In the lab |
| --- | --- |
| Caddy with real certificates | `cmd/front`: HTTPS on 443 with a lab CA (`.run/ca.pem`, key in `.run/ca-key.pem`). Panel and relay get a lab certificate; protected sites get the one uploaded in Site → Certificate, fetched from the gateway like Caddy does (`/tls/certificate`). Port 80 redirects the lab names to https and sends protected sites to the gateway's plain port 18084 |
| The customer's identity provider | `cmd/idp`: OIDC provider with two users, alice and bob |
| The discovery agent | `python3 -m equalizer_agent` from this checkout, logs in `.run/logs/agent.log` |
| An SMTP server | `smtp_sink.py`: every email saved in `.run/mail/` |
| DNS | a line in `/etc/hosts` marked `# equalizer-lab` |
| Docker networks | everything listens on 127.0.0.1 only |

Names: `panel.lab.test`, `relay.lab.test`, `idp.lab.test`, and ten
protected sites, each a real application served by PHP's own server:

| Site | Application | Legacy address |
| --- | --- | --- |
| `www.acme.test` (aliases `acme.test`, `old.acme.test`) | WordPress 6.9 on SQLite | 127.0.0.1:18090 |
| `news.acme.test` | Joomla 5.4 on PostgreSQL | 127.0.0.1:18091 |
| `docs.acme.test` | Drupal 11.4 on SQLite | 127.0.0.1:18092 |
| `intranet.acme.test` | Acme Intranet, an old-style PHP app in `apps/intranet` (sessions, ISO-8859-1 pages, attachments), published in plain http with no certificate | 127.0.0.1:18093 |
| `cart.acme.test` | OpenCart 4.0 on MariaDB, with its demo products | 127.0.0.1:18101 |
| `shop.acme.test` | PrestaShop 8.2 on MariaDB, with its demo products | 127.0.0.1:18102 |
| `forum.acme.test` | phpBB 3.3 on SQLite | 127.0.0.1:18103 |
| `wiki.acme.test` | MediaWiki 1.43 on SQLite | 127.0.0.1:18104 |
| `site.acme.test` | TYPO3 13.4 on MariaDB, three pages | 127.0.0.1:18105 |
| `store.acme.test` | Magento 2.4.7 on MariaDB, without its search modules (no OpenSearch here) and without products | 127.0.0.1:18106 |

Joomla and Drupal come from GitHub only: their dependencies are installed
from each project's `composer.lock` (`cms.sh`). The first `up.sh` builds
them in `.run/src` (a few minutes); later runs reuse the build. In the lab
Joomla has no update checks or guided tour (no Internet), and Drupal is
installed from its recipes one by one.

The six sites after it prove the agent's other profiles (test 6 below) and
are prepared by `webapps.sh`, from GitHub too: release archives (OpenCart,
PrestaShop) or clones with dependencies from their lock files (phpBB,
MediaWiki, TYPO3, Magento; Magento without Composer plugins, one of them
asks Packagist). The first run takes about 20 minutes more, Magento most of
it. MariaDB keeps its data in `/var/tmp/equalizer-lab-mysql` and listens on
127.0.0.1:33061. Their test accounts: a customer `editor@acme.test` on the
three shops, `editor` on the forum and the wiki (all with the password
below); TYPO3 has no visitors' login.

## Requirements

Go 1.24, Node 20, PHP 8.3 with `pdo_sqlite`, `pdo_pgsql`, `mysqli`, `gd`,
`intl`, `zip`, `bcmath` and `soap`, Composer, PostgreSQL 16, MariaDB 10.11,
Redis, git, zip and unzip,
Python 3 with `pytest`, `requests`, `playwright` and `aiosmtpd`, Google
Chrome (`/usr/bin/google-chrome`), and root (ports 80/443 and
`/etc/hosts`).

## Use

```sh
sudo ./up.sh                 # build, start, create the platform admin
python3 -m pytest -q tests   # about 15 minutes (the alert test waits for the grace period)
./down.sh                    # stop; --clean also removes the data and the hosts line
```

Admin: `root@lab.test` / `Lab-root-password-1` (TOTP enrolled by the
tests). Every legacy site: `editor` / `Lab-editor-pass-1`. Secrets are generated
in `.run/`, which is never committed. Logs are in `.run/logs/`.

The tests run in order and build on each other:

1. **onboarding**: admin login, tenant, invitation by email, tenant owner
   and approver, connector online at the platform's release, app with
   aliases, WordPress template, approval and publication;
2. **wordpress**: links in https, accents, pages and search, comments,
   alias redirects, login (no legacy cookie in the browser), remember me,
   REST with nonce, media upload (raw body and form), no mixed content in
   wp-admin, settings saved with WordPress's own address, an article
   published from the block editor;
   then **fidelity**: about 40 public and admin pages of the four sites
   seen directly and through Equalizer, compared pixel by pixel, by
   visible text and by resources that fail to load (before SSO, which
   changes the WordPress login page);
   then **the other sites**: Joomla (home, article, Smart Search, an
   article written in the admin with its editor), Drupal (page, search,
   an article written with CKEditor, admin pages) and the intranet
   (accents both ways, search, login, a notice with an attachment, a PHP
   script disguised as a picture refused);
3. **attacks**: traversal, XSS, XXE, a PHP file sent as an image, unknown
   bodies, forged and hidden headers; then **hostile clients**, written
   byte by byte on the wire: paths that climb on Tomcat and Windows,
   request smuggling, huge headers, other hosts, forms with too many
   fields, forged client addresses, a session flood from one address and
   a body sent at a drip; then **relay and panel**: guessed connector
   tokens, wrong panel passwords (one account locked, not everyone),
   cross-site writes, the internal API, oversized bodies;
4. **sso**: OIDC on `/wp-admin/`, `/wp-login.php` and `/xmlrpc.php` only, public pages
   without login, allowed and refused users, tricks around the paths;
5. **operations**: password reset by email, alert when the connector
   stops and when it comes back;
6. **study**: the discovery agent (started by `up.sh`, through the relay
   and the connector) studies each of the ten sites with its test account
   (the customer on the shops); each site's content in its database is the
   same before and after, every draft is verified on the gateway, TYPO3's
   study says it found no visitors' login, and Drupal's and Joomla's administrations become
   precise entries of the draft (WordPress's stays closed: it writes when
   its screens are only viewed).

`python3 fidelity.py [host…]` runs the comparison alone (on a lab without
SSO, such as right after test 2) and writes side-by-side
images (the site directly on the left, through Equalizer on the right) to
`.run/fidelity/`.

## Not covered

Real TLS and ACME, DNS, Docker Compose images, real SMTP delivery,
Cloudflare in front, the Logs page (Loki), `backup.sh`/`restore.sh`, more
than one gateway or relay replica. These stay for the manual test on a
server.
