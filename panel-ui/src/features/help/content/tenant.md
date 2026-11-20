# Equalizer guide for customers

For whoever protects their own sites with Equalizer. The same guide opens
in the panel under **Help**. Screenshots use a fictitious company, Acme.

## Getting started

### What Equalizer does

Equalizer stands in front of a web application that cannot be updated. The
legacy server is no longer reachable from the Internet: a small program on
its network, the **connector**, opens an outbound connection to Equalizer,
and Equalizer serves the site on its usual domain. Visitors see the same
site; every request they send is checked and written again before it
reaches the legacy application, so malformed or malicious input never
arrives as it was typed.

What the site may receive is described by its **map**: by default the
whole site, admin included, plus precise checks on the pages the map knows
well. Maps are versioned, reviewed and approved by people of your
organisation.

### The panel

![Protected sites](/help-img/sites.png)

- **Protected sites**: your sites, each with a dot (green protected, grey
  being set up, red exposed), and **+ Protect a site**.
- **Status**, **Connectors**, **People**, **Activity**, **Logs**,
  **Settings**: some pages depend on your role.
- A site has four tabs: **Map**, **Discovery**, **Isolation**, **Login**.
- At the bottom of the sidebar: your name and role (opens your account),
  **Help** (the question mark: this guide) and **Log out** (the arrow out
  of the door).

### Roles

**Administrator** (everything), **editor** (writes maps), **approver**
(approves and publishes), **auditor** (reads). Whoever writes a version
cannot approve it.

| Page | Administrator | Editor | Approver | Auditor |
| --- | --- | --- | --- | --- |
| Sites, Status, People | yes | yes | yes | yes |
| Connectors, Settings, Isolation | yes | | | |
| Activity | yes | | yes | yes |
| Map: write drafts, import | yes | yes | | |
| Map: approve, publish, roll back | yes | | yes | |
| Login: settings (as a draft) / client secret | yes / yes | yes / | | |
| Logs | yes | | yes | yes |

### Joining and inviting

The first administrator receives an email from the platform with a link to
choose a password and set up an authenticator app. Then **People → Invite a
person** (name, email, role): the link works once and expires in 48 hours;
pending invitations can be revoked.

![People](/help-img/people.png)

### Forgotten password

**Forgot your password?** on the login page sends a link to the account's
email; it works once, for one hour, and logs out every browser of the
account. The authenticator app stays as it is. A lost authenticator is
reset by an administrator of your organisation (**People**).

### Privacy

The platform operator sees your organisation's name and, per site, its
name and status (protected, being set up, exposed). Nothing else: not your
maps, connectors, people, logs or activity.

## Protecting a site

### The steps

The original site keeps working as it is during the whole procedure.

1. Add the site.
2. Install a connector on its server.
3. Write the first map (from a template).
4. Optionally run discovery.
5. Upload the site's certificate (or choose http, if the site never had one).
6. Have the map approved and publish it.
7. Point the domain to Equalizer and isolate the old server.

### Add the site

**+ Protect a site**: name, public domain (e.g. `portal.acme.example`) and
what it is built with (WordPress, Joomla, Drupal, Moodle or another site).
The choice lets the first map start from a template.

### Install a connector

One connector per server: it serves every site on that server.
**Connectors → Add a connector** gives a token, shown once, and the command
to run on the server (Docker or a binary). The connector only opens an
outbound HTTPS connection: no inbound port. Its `EQ_ALLOW` lists the
addresses it may reach; nothing else is reachable through it, whatever it
is asked.

The **Connectors** page shows whether each one is online and its release.
A connector older than the platform is marked **Older than the platform:
update it** (also on the Status page): some features do not work through
it, e.g. the site's own HTTP login. The page gives the command with the
right release.

![Connectors](/help-img/connectors.png)

### Write the first map

**Site → Map → New version**:

- **Start from a template**: choose the connector and the address of the
  site on the server; the editor fills with the pages, files and forms of
  that CMS. The whole site works at once, admin included. **Max upload
  (MB)** is the largest file the site accepts (default 50, at most 1024).
- **Import a map file**: a YAML or JSON map exported from another site.
- Or write it in the editor (see **Maps → What a map contains**).

![New version](/help-img/new-version.png)

**Save draft** checks the format, that the connector is yours, that the
address is allowed and that the gateway would accept the map. Then **Send
for review**.

### Discovery

**Site → Discovery → Start discovery**. The agent explores the site through
the connector, never from the Internet, and follows no link that changes
something. With test accounts (up to three,
for example a customer and an administrator) it also explores as each of
them, in a separate pass, to reach the pages behind their login. With an
administrator account it also studies the administration, read-only
(Drupal, Joomla): it opens no link or screen that changes something.
WordPress's administration stays closed to it, because WordPress writes to
its database when its screens are only viewed. Give
accounts with the fewest permissions that still see those pages; the
passwords are encrypted and deleted at the end. The map does not restrict
pages by account: the site itself still decides who sees what.

The agent reads each form's fields as the page declares them (type,
length, limits, pattern, choices) and gives each the narrowest check that
allows: a list of values, a number with its range, an email, a phone, a
pattern. Free-text fields refuse HTML and template code; fields holding a
name or a code also refuse SQL delimiters.

![Discovery](/help-img/discovery.png)

The whole site always stays in the map through the `/` area. The report
shows what was found, which pages got precise entries, how each test
account logged in, what is **still open** with its reason (pages not
opened and still served by the area, links that change something, fields
that must still accept SQL characters such as a search or a message), what
visitors asked for that the map lacks, and the protection against IDOR it proposes for JSON APIs (a list
and a detail with the same id). If the map should change, a draft is
proposed: review it and send it for approval. **Continuous discovery**
(every day or week) explores again and proposes a draft only when something
changed.

### Verification

Every draft a discovery proposes is **verified** by the agent on the
gateway, and **Verify** on a version (editors, approvers, administrators)
does it again, for example after changing a draft by hand. Nothing is sent
to the site that could change it:

- every page the discovery studied is opened directly and through
  Equalizer with this version, and the two are compared (the answer, where
  a redirect goes, the visible text, the links, the forms);
- every parameter and form field gets a set of attacks (SQL, HTML and
  script, template code, paths, shell, out-of-type values); the gateway
  only says whether it would let each one through.

The panel shows how many pages stay identical, how many inputs refuse
every attack, and how much of the site only the generic area serves; then
the pages this version breaks and what is still open, each with its
reason. An input "still open" usually means that a value falling out of
its precise entry is served by the site area. Pages behind a login are not
verified yet.

![Verification](/help-img/verification.png)

### Review and approval

Each version shows what it changes against the published map and the risks
found by fixed rules (a type widened, an admin or backup path, a page with
personal data and no authorisation rule, an answer that grants access to
more objects…). With an AI model a summary and the questions to ask appear
too. Approving stays a person's decision.

![Review aid](/help-img/review.png)

**Approve**, then **Publish**: the gateways load the signed map within 15
seconds.

### The site's certificate

Equalizer never makes a certificate for a protected site: visitors see the
site's own, the one its server uses today. In **Site → Certificate** an
administrator uploads it:

- a **.pfx** file (IIS exports it this way, with a password), or
- the **PEM** files or text: the certificate, its chain (intermediate
  certificates) and its private key; an encrypted key asks for its password.

Before keeping it the panel checks that the key matches, that the chain is
complete (a public CA, or your company's CA if you upload it too), that it
is valid today and that it covers the site's main domain. The page shows
the domains it covers, the other domains it does not cover (those get no
HTTPS), its issuer and its end. The private key is kept encrypted and never
shown again; every administrator of the company gets an email when the
certificate is uploaded, replaced or removed. The gateways use a new
certificate within seconds.

Upload it **before** pointing the domain: then the switch has no gap. A
site published without a certificate gets no HTTPS (the panel warns at
publication) until one is uploaded. When the certificate is near its end,
**Status** turns yellow and the administrators get an email 30, 14, 7 and 1
days before; upload the renewed one in the same place.

A site that never had a certificate can stay in **http**, as before
Equalizer (**Site → Certificate → Serve it in http**): pages and logins
travel in clear, and the login with an identity provider is not available.
A site in https sends visitors who type `http://` to `https://`.

![Certificate](/help-img/certificate.png)

### Point the domain and isolate the site

1. Point the site's DNS records to the platform (the address is given by the
   platform operator): the main domain and its other names (the bare
   domain next to `www`, old domains). In https the site's certificate must
   cover them. In **Site → Isolation → Other domains** list the other
   names: visitors on them are sent to the main domain, as sites usually
   do. They apply at once, without a new version of the map.
2. **Site → Isolation**: enter the old public IP of the site. Equalizer
   checks regularly that the site no longer answers there, and gives the
   lines to copy for Apache, nginx or IIS: move the site to an internal
   address, or let only the connector in. The other sites on the server stay
   as they are.

![Isolation](/help-img/isolation.png)

If a protected site becomes reachable again on its old IP, it turns red on
the Status page and in Activity.

## Maps

### What a map contains

A map is a YAML document (the full format is in `docs/map-format.md` of the
project):

- **legacy**: where the legacy site is and which connector reaches it.
- **roles**: the visitor's role before and after logging in.
- **areas**: whole sections served as the legacy site serves them; `/` is
  the whole site. Every request is still rebuilt.
- **routes**, **forms**, **apis**, **assets**: precise entries with typed
  values (an id is a number, a slug has no markup…), sealed forms, rate
  limits. Where they match, they take precedence over the area.
- **defaults**: limits, session duration, "remember me".

A map with only the `/` area already protects the site; precise entries add
stricter checks where you know the pages well.

### Versions and rollback

Every change is a new version: draft, in review, approved, published.
**Site → Map** lists them; each can be downloaded with **Download as YAML**.

![Versions](/help-img/maps.png)

If a new version breaks the site, open the previous version and **Roll back
to this version**: it is live again within 15 seconds.

### Import and export

**Download as YAML** exports a version; **Import a map file** (in **New
version**) opens a YAML or JSON map in the editor. Tenant, site and domain
are set from the site you import into, and the draft gets the usual checks
and approval.

## Login with your identity provider

### Turning it on

Off by default. **Site → Login** turns on the login with your
organisation's identity provider (Entra ID, Google, Okta, Keycloak…) before
the site: issuer, client ID, and which group of your provider gets which
role of the map. **Save as new draft** writes a new version that follows
the usual review and approval.

![Login](/help-img/login.png)

### Where the login is asked

- **The whole site**: every page asks for the login first. For intranets
  and internal applications.
- **Only some paths**: the admin and login pages ask for it, the public
  site stays open. **Use the WordPress paths** (or Joomla, Drupal, Moodle)
  fills the list: for WordPress `/wp-admin/`, `/wp-login.php` and
  `/xmlrpc.php`. Spellings like `/WP-ADMIN/` or `//wp-admin/` are matched
  too.
- **Optional**: nobody is sent to the login; visitors log in at
  `/.eq/oidc/login`.

After the identity provider, visitors still log in to the site with its
own account (two logins): the provider says they are one of yours, the
site's login says what they may do inside it. Until they pass the
provider, the site's HTTP login (e.g. WordPress application passwords)
never reaches the site.

### Client secret and redirect URI

In your provider register the redirect URI shown on the page, and save the
client secret in **Site → Login** (administrators only; encrypted, never
shown again). The site's own login keeps working behind it.

## Monitoring

### Status

Whether each site works, and which part is broken: published map,
connector, legacy site (errors in the last 15 minutes), isolation, and the
last 7 days. Refreshed every 30 seconds.

![Status](/help-img/status.png)

### Logs

What the gateway served and refused on your sites, for 30 days: filters by
site, period, kind of event and free search (IP address, path, request ID).
A row opens its detail in plain words; on a path that is not in the map,
**Add to the map** opens a new version. **Live** refreshes every few
seconds; **Export CSV** downloads up to 10 000 events (recorded in
Activity). Field values are never stored, only the field name and the rule
that refused it. The IP address is the visitor's real one, the same the
legacy site receives in `X-Forwarded-For` (its own logs and bans keep
working).

![Logs](/help-img/logs.png)

### Activity

Who did what: logins, invitations, maps written, approved, published, rolled
back, exports.

![Activity](/help-img/activity.png)

## Settings

### Alerts

**Settings → Alerts**: an email when a site stops working for more than a
few minutes (connector offline, legacy site failing, site reachable again
on its old IP) and when it works again. It goes to the administrators of
your organisation and to up to five more addresses (an on-call mailbox, a
ticketing address). On by default; each change is sent once.

### AI model

**Settings → AI**: your own model (Claude, OpenAI or your own server) names
routes, spots sensitive pages and narrows types during discovery, and
summarises versions for the approver. The key is yours, encrypted and never
shown again. Without a model discovery still works.

![Settings](/help-img/settings-ai.png)

## What Equalizer protects

### The whole site, rebuilt

The whole site works as before, admin included, with its logins,
"remember me", uploads (up to the size chosen in the template), live
updates and mobile version. Equalizer does not decide who may do what, the
legacy site does. It removes the direct contact with the legacy server and
rebuilds every request: path, query, forms, JSON, XML and text are parsed
and written again; malformed requests, XML entities and unknown binary
bodies never reach it.

### Typed fields and rate limits

Where the map has precise entries (templates and discovery add them):
no SQL or HTML characters where not expected, sealed forms, rate limits and
behaviour rules against brute force and scanners.

### Authorisation per object

With `owned_by_user`, each user opens only the objects the site showed
them, in its pages, redirects or JSON answers; changing the number in the
address gives 403.

### Sessions, cookies and headers

The legacy site's session and login cookies stay on Equalizer: the browser
holds only a random id. Other cookies (language, consent, those written by
the page's scripts) work as usual. Only known headers reach the legacy
site; those a server could trust as set by a proxy are dropped. The legacy
site receives the visitor's real IP and a standard User-Agent for the kind
of device.

### Uploads

Real type read from the bytes, size limit, random name, images rebuilt
from their pixels. Files sent by JavaScript uploaders as the whole request
(not as a form) are checked the same way: their bytes must be what they
claim, and pages, scripts and programs are refused. Equalizer does not scan for viruses: keep your antivirus
or EDR on the legacy server.

## Troubleshooting

### The site does not answer

Open **Status**: it says which part is broken. Connector offline: the
connector machine or its network (restart the connector). Legacy site
errors: the legacy application itself. Map: no published version.

### A page or action is refused

Open **Logs** and search the path. The detail says why: a value refused by
a precise entry (widen its type in a new version), a rate limit (429), an
object not shown to that user (403), a method the area does not allow
(405). A path not in the map has **Add to the map**.

### An upload fails

"Body too large" (413) in Logs: raise the upload size of the area
(`max_body_kb`, or start a new version from the template with a larger
**Max upload**). "File not what it claims" (415): a file sent as the whole
request did not match its type, or was a page, a script or a program.
Other bodies that are neither forms, JSON, XML, text nor files are refused
(415).

### Live features do not work

Live updates (`text/event-stream`) work. WebSocket is not supported yet:
the gateway answers 501 and the page falls back or shows an error.

## Not available yet

### Planned

- Forwarding to your SIEM (Splunk, Sentinel, syslog).
- Capacity settings (replicas, cache) from the panel.
- Single sign-on to the panel itself, and one legacy account per user behind
  the identity provider login.
- Alerts for known vulnerabilities of the detected CMS version.
- WebSocket.
