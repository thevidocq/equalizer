# Local end-to-end lab

A whole Equalizer platform on one machine, without Docker, to test what a
customer does before a manual test on a real server. The services are the
real ones, built from this checkout; the legacy site is a real WordPress.
What only exists on a VPS is replaced by small stand-ins:

| On a VPS | In the lab |
| --- | --- |
| Caddy with real certificates | `cmd/front`: HTTPS on 443 with a lab CA (`.run/ca.pem`) |
| The customer's identity provider | `cmd/idp`: OIDC provider with two users, alice and bob |
| An SMTP server | `smtp_sink.py`: every email saved in `.run/mail/` |
| DNS | a line in `/etc/hosts` marked `# equalizer-lab` |
| Docker networks | everything listens on 127.0.0.1 only |

Names: `panel.lab.test`, `relay.lab.test`, `idp.lab.test`, and four
protected sites, each a real application served by PHP's own server:

| Site | Application | Legacy address |
| --- | --- | --- |
| `www.acme.test` (aliases `acme.test`, `old.acme.test`) | WordPress 6.9 on SQLite | 127.0.0.1:18090 |
| `news.acme.test` | Joomla 5.4 on PostgreSQL | 127.0.0.1:18091 |
| `docs.acme.test` | Drupal 11.4 on SQLite | 127.0.0.1:18092 |
| `intranet.acme.test` | Acme Intranet, an old-style PHP app in `apps/intranet` (sessions, ISO-8859-1 pages, attachments) | 127.0.0.1:18093 |

Joomla and Drupal come from GitHub only: their dependencies are installed
from each project's `composer.lock` (`cms.sh`). The first `up.sh` builds
them in `.run/src` (a few minutes); later runs reuse the build. In the lab
Joomla has no update checks or guided tour (no Internet), and Drupal is
installed from its recipes one by one.

## Requirements

Go 1.24, Node 20, PHP 8.3 with `pdo_sqlite`, `pdo_pgsql`, `gd`, `intl` and
`zip`, Composer, PostgreSQL 16, Redis, git,
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
   bodies, forged and hidden headers;
4. **sso**: OIDC on `/wp-admin/`, `/wp-login.php` and `/xmlrpc.php` only, public pages
   without login, allowed and refused users, tricks around the paths;
5. **operations**: password reset by email, alert when the connector
   stops and when it comes back.

`python3 fidelity.py [host…]` runs the comparison alone (on a lab without
SSO, such as right after test 2) and writes side-by-side
images (the site directly on the left, through Equalizer on the right) to
`.run/fidelity/`.

## Not covered

Real TLS and ACME, DNS, Docker Compose images, real SMTP delivery,
Cloudflare in front, the Logs page (Loki), `backup.sh`/`restore.sh`, more
than one gateway or relay replica. These stay for the manual test on a
server.
