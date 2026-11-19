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

Names: `panel.lab.test`, `relay.lab.test`, `idp.lab.test`, and the
protected site `www.acme.test` (aliases `acme.test`, `old.acme.test`).

## Requirements

Go 1.24, Node 20, PHP 8 with `pdo_sqlite`, PostgreSQL 16, Redis, git,
Python 3 with `pytest`, `requests`, `playwright` and `aiosmtpd`, Google
Chrome (`/usr/bin/google-chrome`), and root (ports 80/443 and
`/etc/hosts`).

## Use

```sh
sudo ./up.sh                 # build, start, create the platform admin
python3 -m pytest -q tests   # about 8 minutes (the alert test waits for the grace period)
./down.sh                    # stop; --clean also removes the data and the hosts line
```

Admin: `root@lab.test` / `Lab-root-password-1` (TOTP enrolled by the
tests). WordPress: `editor` / `Lab-editor-pass-1`. Secrets are generated
in `.run/`, which is never committed. Logs are in `.run/logs/`.

The tests run in order and build on each other:

1. **onboarding**: admin login, tenant, invitation by email, tenant owner
   and approver, connector online at the platform's release, app with
   aliases, WordPress template, approval and publication;
2. **wordpress**: links in https, accents, pages and search, comments,
   alias redirects, login (no legacy cookie in the browser), remember me,
   REST with nonce, media upload (raw body and form), no mixed content in
   wp-admin;
3. **attacks**: traversal, XSS, XXE, a PHP file sent as an image, unknown
   bodies, forged and hidden headers;
4. **sso**: OIDC on `/wp-admin/`, `/wp-login.php` and `/xmlrpc.php` only, public pages
   without login, allowed and refused users, tricks around the paths;
5. **operations**: password reset by email, alert when the connector
   stops and when it comes back.

## Not covered

Real TLS and ACME, DNS, Docker Compose images, real SMTP delivery,
Cloudflare in front, the Logs page (Loki), `backup.sh`/`restore.sh`, more
than one gateway or relay replica. These stay for the manual test on a
server.
