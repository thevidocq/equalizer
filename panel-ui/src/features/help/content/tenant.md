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
well. Maps are versioned, reviewed and published by an administrator of your
organisation.

### The panel

![Protected sites](/help-img/sites.png)

- **Protected sites**: your sites, each with a dot (green protected, grey
  being set up, red exposed), and **+ Protect a site**.
- **Status**, **Connectors**, **People**, **Activity**, **Logs**,
  **Settings**: some pages depend on your role.
- A site has these tabs: **Discovery**, **Map**, **Isolation**,
  **Certificate**, **Login**, **Security**, **Source**. Discovery comes first: it is where a new site
  starts. **Ask AI** in a site's header opens [the assistant](#the-assistant).
  An administrator deletes a site from the list of **Protected sites**
  (**Delete** on its row); it asks you to type the site's name to confirm,
  and removes the map, connectors, discoveries and certificate with it.
- At the bottom of the sidebar: your name and role (opens your account),
  **Help** (the question mark: this guide), **Log out** (the arrow out
  of the door) and the release of Equalizer, which opens its release notes.

Dates and times are written the same way everywhere, for example
`21 Nov 2026, 02:33:07 UTC+1`, in the time zone chosen in **Your account**
(UTC until you choose one). Alert emails use the same time zone.

### Roles

Two roles. An **administrator** does everything and publishes a map alone,
without a second person. A **viewer** reads and changes nothing, for
example an auditor or a manager who follows the sites.

| Page | Administrator | Viewer |
| --- | --- | --- |
| Sites, maps, Status, People, Activity, Logs | yes | reads |
| Connectors, Settings, Isolation | yes | |
| Map: write, publish, roll back, import | yes | |
| Login: settings and client secret | yes | |
| Assistant: ask | yes | yes |
| Assistant: make a proposed change a draft | yes | |
| Security: read scans and findings | yes | yes |
| Security: scan, connect the scanner | yes | |

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

1. Add the site. It opens on **Discovery**.
2. On the first discovery, choose how Equalizer reaches the site: install a
   connector on its server, or reach it directly at its public address.
3. Run discovery: the agent explores the site and proposes a draft map.
4. Review the proposed map on the **Map** tab (or write one by hand from a
   template).
5. Upload the site's certificate (or choose http, if the site never had one).
6. Publish the map.
7. Point the domain to Equalizer and isolate the old server.

Discovery first, then the map: the agent does the exploring and hands you a
draft to review, instead of starting from an empty map. You can still build
a map by hand on the **Map** tab if you prefer.

### Add the site

**+ Protect a site**: name, public domain (e.g. `portal.acme.example`) and
what it is built with (WordPress, Joomla, Drupal, Magento, PrestaShop,
OpenCart, TYPO3, Moodle, MediaWiki, phpBB, or another application or a
custom site). The choice lets the first map start from a template and tells
the discovery agent what to expect; what the agent does for each one, and
what the lab has proven, is in the
[compatibility matrix](../compatibility.md).

The name can be changed later: the pencil next to it at the top of the
site (administrators) renames it. Only the name shown in the panel and in
emails changes: the domain, the site's address in the panel and the map
stay the same. Activity records the old and the new name.

![Rename the site](/help-img/rename.png)

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

### Or reach the site directly

When the site stays on the Internet at its own address and you control who
may reach it, the connector is not needed: in the template or the first
discovery choose **Directly, at the site's public address** and enter its
address and port. This is a public IP (for example `203.0.113.10:443`) or a
name that resolves to public addresses only (for example
`origin.example.com:443`, a site behind a CDN whose IP you should not pin).
Equalizer connects only to a public address: a name that resolves to a
private or reserved one is refused, at save time and again at the moment of
connection, so it can never reach the platform's own network. The panel
shows the address Equalizer connects from: in production the site must
accept only that one and refuse everyone else, or visitors could skip
Equalizer (for a test it does not matter). Use HTTPS: the requests cross the
Internet. A site on a private network always needs a connector.

**The name the site is asked for.** Equalizer sends the legacy site a name
(the `Host` header and the TLS name) and checks the site's certificate
against it. You do not choose it:

- with a **name** in the address (`www.example.com:443`), Equalizer asks for
  that name — what a site behind Cloudflare or another CDN needs;
- with an **IP** in the address, or through a connector, it asks for the
  site's public domain.

Under **Advanced** you can set another name (*Name asked of the site*), only
for a site that answers to a name different from both — for example a
dedicated origin hostname whose server only knows the public name.

### A parallel domain, to try a site without moving it

You can put Equalizer in front of a site under **another name**, while the
original keeps working as it is. For example `test.example.org` through
Equalizer, in front of `www.example.com` (here behind Cloudflare):

1. **DNS of your test domain:** `test.example.org` → the VPS address, proxied
   by Cloudflare (orange cloud).
2. **Certificate for the test domain:** in Cloudflare (zone of
   `example.org`) → SSL/TLS → *Origin Server* → *Create certificate* for
   `test.example.org`; SSL mode **Full (strict)**. Any valid certificate for
   that name works too.
3. **Protect a site** with domain `test.example.org`.
4. **Discovery:** *Directly*, address `www.example.com:443`, HTTPS. Equalizer
   asks the site for `www.example.com` by itself.
5. **Site → Certificate:** upload the certificate and key of step 2.
6. Review the proposed map and **Publish**; open `https://test.example.org`.

Visitors of `www.example.com` are not touched. Links in the pages are
rewritten to the test domain, so you stay on it while browsing.

**Changing it later.** How Equalizer reaches a site — through a connector or
directly, which connector, which address — is chosen the first time
discovery runs. Afterwards you change it in **Site → Isolation**, under
*How Equalizer reaches this site*: the current map is republished with the
new reach and your tuned entries are kept. You no longer rerun discovery to
change it.

### Write the first map

**Site → Map → New version**:

- **Start from a template**: choose how the site is reached (the connector
  and the address on the server, or its public address); the editor fills
  with the pages, files and forms of
  that CMS. The whole site works at once, admin included. **Max upload
  (MB)** is the largest file the site accepts (default 50, at most 1024).
- **Import a map file**: a YAML or JSON map exported from another site.
- Or write it in the editor (see **Maps → What a map contains**).

![New version](/help-img/new-version.png)

**Save draft** checks the format, that the connector is yours, that the
address is allowed and that the gateway would accept the map. Then
**Publish**.

### Discovery

**Site → Discovery → Start discovery**. The agent explores the site the
way Equalizer reaches it, through the connector or directly at the
address that accepts only Equalizer's, and follows no link that changes
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

The AI is always used when your organisation has a model (Settings → AI
model); without one, a soft amber notice says so above the button, while
the discovery runs and in its report, and the draft comes from the fixed
rules only. To stop using AI, remove the model in Settings. **Test
accounts** and **Pages at most** (500 by default) are closed under their
arrow; once filled, the closed line says how many accounts are set.

**Read the site's source code** (checked by default once the site's
[source reader](#the-source-reader) answers; grey until then): the
discovery first has the reader read the site's code, and waits for it, 10
minutes at most. A field the agent would leave as free text gets the
code's type when the code always reads it as a number, an email or one of
a few values, and the values seen on the site fit; a field the code reads
as it is anywhere stays free text. The report gains **From the source
code**: the fields typed by the code, the entry points no link shows (REST
routes, AJAX actions, PHP files that run when opened: the site area serves
them as they are) and the calls where a request value may do harm, with
file and line, each grouped by plugin or theme, closed but for its count;
a long list says "first 200 of N". The components it finds join the
report's one **Components** list, which says of each whether the pages,
the code or both showed it. If the read fails or takes longer, the
discovery runs without it and says why.

It also finds every file the pages load (stylesheets, scripts, images,
fonts, videos, documents), including those named only in stylesheets,
lazy-loaded images and video posters, and the options their links use (a
sort, a filter: `?orderby=`), and proposes them in the map. To a folder
already in the map it only ever adds passive types (images, fonts, audio,
video, stylesheets, PDF and office documents; scripts a page loads): never
text, CSV, archives, HTML or XML, which would open every such file in the
folder.

**What visitors asked for.** Equalizer notes what it refused visitors: a
file, a page option (a sort, a filter: `?orderby=`). A visitor's request is
not proof of anything — anyone can ask for anything, and scanners try
thousands of addresses — so the agent puts one in its draft only after
checking it on the site:

- a **file** goes in when at least two different visitors asked for it, it
  is of a harmless type (image, font, audio, video, stylesheet, PDF or
  office document) in a folder the map already serves, and the agent finds
  it on the site, of that type. A `.php` file, a script or a page never
  goes in, however many ask. Files the site's own pages use are added by
  the study anyway.
- a **page option** goes in when at least two different visitors used it,
  it changes how a page presents its content (a sort, an order, a page
  number, a view, a language, a shop filter), the site itself offers its
  values (the choices of its menus and links, the same on every page that
  offers them), and the page really shows something else with it. It goes
  in with **only those values**, or as a whole number for page numbers and
  for prices the site offers as whole numbers: any other value, an attack
  included, is refused. An option the site ignores (`?test=1`), one that
  only redirects or errors, one whose values the site does not offer, and
  any other kind of key (which could make the site do something, such as
  adding to a cart) never go in — and the agent never opens the latter.

What went in is listed in the report under **Added from visitors'
requests**, with the reason, and in the email that tells you the draft is
ready: look at it before publishing, and take out of the draft what you do
not want. Scanners, a single visitor, files that do not exist, scripts and
options that change nothing never appear (they stay in the logs). One
visitor coming back many times still counts as one: Equalizer counts
visitors by a key made from their address with a secret only the gateway
knows, never storing the address itself. Adding a file type serves every
file of that type in that folder.

![Discovery](/help-img/discovery.png)

While it runs, the page shows how many pages it has found and, under it,
what it is doing now: exploring (as which test account) with the page it
is on, opening pages in the browser to see what their scripts load,
reading the stylesheets, building the map, the AI studying the structure,
checking what visitors asked for.

**The security study** is the discovery's conclusion, first in the
report. It is written **after the agent's check of the draft**, on the map
as the check leaves it (corrections included): until then the report says
so, and the study appears by itself when it is ready, with the version it
was written on. If the discovery has no draft, it starts right away; if
the check never ends, it starts three hours after the discovery. The AI reads everything the discovery found, on the surface and
in the code: every page, form and API entry with the type of each field,
the components and their versions, what stayed open, what visitors asked
for, and, when the source code was read, the entry points only the code
shows and the risky calls (an SQL query, a file written, a command run)
with the field that reaches them and their file and line. It studies them
together, as a security engineer and a developer would, and writes:

- a **summary**: what the site is, how it is exposed, what matters most;
- the **attack surface** in numbers (counted by the agent, not by the
  AI);
- the **first steps**, in order;
- the **findings**, most urgent first, each with its severity (a dot and
  a word: critical, high, medium, low), why, what an attacker would try,
  how far the map stops it once published (stopped, partly stopped,
  open), the next step, and the pages, code places and components it is
  about. A next step is either something Equalizer does (narrow a field,
  cover an entry point precisely by running the discovery with the source
  code, a rate limit, an owner check) or something outside it (update a
  component, read the code at a given line, a task **on the site
  itself**). The study respects Equalizer's choices: the whole site is
  served, administration included, and who may reach it is the site's
  decision, so it never advises closing an area or a login page at the
  gateway; a field left free because the site needs those characters (a
  message with quotes, a rich-text field) stays free, and the study says
  what the site's own code must do with it.

It works on names, types and places only: never a value, a page's text,
a line of code or a secret. Every finding cites what the discovery found;
Equalizer drops any that cites something else. It changes nothing in the
draft: the assistant can turn a "narrow a field" into a draft for you. What
it recalls about a version is to be verified. It needs an AI model
(**Settings → AI**); without one, or if the model does not answer, the
report says so. Without the source code it studies the surface only.

**Pages in the browser.** The agent opens up to 40 pages per pass in a
real browser, to see the requests their scripts make; the panel says which
one (**browser: page 12 of 40**). A page gets 20 seconds at most (the
platform can change it, `EQ_BROWSER_PAGE_SECONDS`): one that is still
loading then, that fails or that crashes the browser is **skipped, never
lost**. The report lists each under **Skipped in the browser** with why
and after how long, and under **Still open** ("its scripts not studied"):
the page itself was explored and mapped, only the requests its scripts
would make were not seen, so the site area keeps serving them without a
precise check. A slow page (a shop's search, a page full of embeds) is the
usual cause. Run the discovery again to retry them; the limit used is in
the report.

The whole site always stays in the map through the `/` area. The report
shows what was found, which pages got precise entries, how each test
account logged in, what is **still open** with its reason (pages not
opened and still served by the area, links that change something, fields
that must still accept SQL characters such as a search or a message), what
it added from visitors' requests, and the protection against IDOR it proposes for JSON APIs (a list
and a detail with the same id). If the map should change, a draft is
proposed: review it and publish it. **Continuous discovery** explores
again every day or every week, at the hour (and day) you choose in your
time zone, and proposes a draft only when something changed; the page shows
the next run and the latest discovery. A schedule saved before you chose
your time zone (in UTC) shows the hour it really runs at in yours, with a
note: **Save** to keep it at that hour in your time zone. The report says whether the AI
model was used; when it was asked but could not answer (no credit left, key
refused, provider not answering) it says so in plain words, and the draft
comes from the fixed rules only.

### Check on the site (verification)

**Discovery finds the pages; the check tries a version on them.** A
discovery explores the site and proposes a map. The check takes one
version of the map, as it is, and measures on the real site what it would
do once published. It changes nothing: not the site, not the published
map, not the version it checks.

What the agent does:

1. It starts a gateway of its own with this version, reaching the site
   the same way the real gateway does (connector or direct).
2. It opens every page the discovery found twice, directly and through
   Equalizer, and compares the two: the answer, where a redirect goes, the
   visible text, the links, the forms. With test accounts it does the same
   behind their login (one pass per account; the passwords are kept
   encrypted and deleted when the check ends). Without them it checks the
   public pages only.
3. On every parameter and form field it tries a set of attacks (SQL, HTML
   and script, template code, paths, shell, values of the wrong type). The
   gateway only says whether it would let each one through: **no attack is
   sent to the site**.

While it runs, the card and the discovery's notice say where it is:
**comparing pages with and without Equalizer: page 120 of 349**, then
**trying attacks on the gateway: entry 14 of 31**, the files, and, when it
corrects its draft, **checking its corrected version**. The agent's log
writes a line every 50 pages. On a site of a few hundred pages it takes
about as long as the discovery.

What you see, in **Check on the site** on each version:

- **pages identical through Equalizer** (for example 34 / 34): the site
  works as before. Fewer means a page breaks; **Broken by this version**
  says which and how (a different answer, missing text, links or forms,
  refused). A page the site itself changes on every load (a shop's random
  related products) is compared on what the site does not vary: the hint
  under the figure says how many there are.
- **inputs that refuse every attack** (for example 0 / 2): the
  parameters and fields where every attack is stopped. The others are
  under **Still open**, each with its reason; usually a value that falls
  out of its precise entry and is served by the site area, or free text
  that has to accept what visitors write.
- **% of the pages served only by the site area**: pages rebuilt and
  served, but with no precise check of their values. Lower is better.

When it runs:

- **By itself after every discovery**, on the draft the discovery
  proposes, and the agent **corrects** it: where the draft breaks a page it
  widens what was refused (or gives the page back to the site area), the
  files the pages load (fonts, images, videos, documents) included; where
  an attack gets through it narrows what the values it saw allow, and
  checks again; it keeps a change only if no page that worked breaks. The
  result is a new draft, "Corrected by the agent from version N", with the
  list of changes and why; the version it came from keeps its own measure.
  After a discovery you review **one draft**, the agent's last: while the
  agent is still checking, the discovery page says so; then it shows the
  draft with its measure and the corrections, and the first draft is
  retired. If the agent could not check its draft, the discovery page says
  so.
- **When you ask** (administrators). You rarely need to: a version a
  discovery made is checked already, and shows only a quiet **Check
  again** under its result. A version never checked (a draft changed by
  hand) shows **Check this version on the site** instead. Ask for a check
  after changing a draft by hand, after the site itself changed without a
  new discovery (an update of the CMS, a new plugin), or to check the
  pages behind a login with test accounts the discovery did not have. Tick **let the agent prepare a
  corrected version** to have the agent fix what it finds into a new
  draft; without it, it only measures. Both versions stay, and a draft you
  have already submitted or approved is never replaced.

Publishing stays a person's decision.

![Verification](/help-img/verification.png)

### Review and publication

Each version shows what it changes against the published map and the risks
found by fixed rules (a type widened, an admin or backup path, a page with
personal data and no authorisation rule, an answer that grants access to
more objects…). With an AI model a summary and the questions to ask appear
too. Publishing stays a person's decision.

![Review aid](/help-img/review.png)

**Publish** (administrators, the author too): the gateways load the signed
map within 15 seconds. Older versions still waiting are retired with it.
**Discard** drops a draft you do not want.

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

**Site → Certificate → How visitors reach the site** has three choices:

- **HTTPS with the site's certificate** (the default): the certificate
  uploaded above; visitors who type `http://` are sent to `https://`.
- **HTTPS handled by a proxy in front**: a CDN, load balancer or reverse
  proxy gives visitors https and reaches Equalizer in http (Cloudflare with
  SSL "Flexible", for example). Equalizer writes every address in https and
  needs no certificate. Between the proxy and Equalizer the traffic is in
  clear. Leave the legacy application's own address in http (WordPress:
  Settings → General). For the visitors' real addresses the platform must
  trust the proxy (`EQ_BEHIND_CLOUDFLARE`, `EQ_TRUSTED_PROXIES`).
- **HTTP, as before**: for a site that never had a certificate; pages and
  logins travel in clear, and the login with an identity provider is not
  available.

![Certificate](/help-img/certificate.png)

### Point the domain and isolate the site

1. Point the site's DNS records to the platform (the address is given by the
   platform operator): the main domain and its other names (the bare
   domain next to `www`, old domains). In https the site's certificate must
   cover them. In **Site → Isolation → Other domains** list the other
   names: visitors on them are sent to the main domain, as sites usually
   do. They apply at once, without a new version of the map.
2. **Site → Isolation**: enter the old public IP of the site (Equalizer's
   own address is refused: the check would reach Equalizer). Equalizer
   checks regularly that the site no longer answers there, and gives the
   lines to copy for Apache, nginx or IIS: move the site to an internal
   address, or let only the connector in. The other sites on the server stay
   as they are. A site that never had a public address of its own (born
   behind Equalizer): **The site never had a public address**, and there is
   nothing to check. A site reached directly is still on the Internet:
   Equalizer cannot check it from its own address, which the site accepts,
   so the page gives a command to run from another network.

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
  limits. Where they match, they take precedence over the area. A request
  that has a page's address but not exactly its shape (an extra parameter
  the map does not know) is served by the area, yet the page's own checks
  still apply: `?id=1' OR '1'='1&x=1` is refused like `?id=1' OR '1'='1`,
  and a page name with angle brackets, braces or quotes (an apostrophe
  inside a word, as in `l'evento`, is fine) is refused too.
- **defaults**: limits, session duration, "remember me".

A map with only the `/` area already protects the site; precise entries add
stricter checks where you know the pages well.

### Versions and rollback

Every change is a new version: draft, then published (or discarded).
**Site → Map** lists them; each can be downloaded with **Download** (see
[Import and export](#import-and-export)).

![Versions](/help-img/maps.png)

If a new version breaks the site, open the previous version and **Roll back
to this version**: it is live again within 15 seconds.

### Import and export

**Download**, on each version, offers four files:

| File | What it is |
| --- | --- |
| YAML | The map itself: **Import a map file** reads it back |
| JSON | The same map, as JSON: it imports too |
| OpenAPI 3 | What the map accepts, for API tools: every page, form and API entry with its parameters, their types, lengths, lists of values and patterns; the file folders; the areas in `x-equalizer-areas`; the roles and rules as `x-equalizer-` extensions |
| ModSecurity / Coraza | Rules for a WAF (ModSecurity v3, Coraza): a request that matches no entry of the map is refused (404), and on each entry a query key, field or file it does not take is refused (403), as is a value outside its type, length, list, pattern or rules |

A WAF checks a request and then forwards it as it came; Equalizer
rebuilds it from the map and never forwards a client's bytes. So the
ModSecurity file keeps the paths, methods, names, types, lengths, lists
of values, patterns, the rules `no_html`, `no_template_meta` and
`no_sql_meta` (approximated), the file types of the folders and the size
of uploads; it loses the rebuilding, the roles and sessions, the checks
of ownership, the rate limits, the ranges of numbers (only digits are
checked), `no_lookalikes`, the rebuilding of images and the rewriting of
the pages. Lengths are counted in bytes there (characters in Equalizer).
Its comments say so; try it with `SecRuleEngine DetectionOnly` first.

**Import a map file** (in **New version**) opens a YAML or JSON map in the
editor. Tenant, site and domain are set from the site you import into, and
the draft gets the usual checks before you publish it.

## Login with your identity provider

### Turning it on

Off by default. **Site → Login** turns on the login with your
organisation's identity provider (Entra ID, Google, Okta, Keycloak…) before
the site: issuer, client ID, and which group of your provider gets which
role of the map. **Save as new draft** writes a new version that follows
the usual review before publishing.

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
A link from the assistant's event card opens the page filtered on that
request. A row opens its detail right under it, in plain words (click it
again to close it; alerts such as a connector going
offline name the connector or the site); on a path that is not in the map,
**Add to the map** opens a new version. **Live** refreshes every few
seconds, and its green dot pulses while it does (**Paused**: grey, still); **Export CSV** downloads up to 10 000 events (recorded in
Activity). **Requests per hour** (per 5 minutes, 6 hours or day, as the
period) has two rows on the same hours: **Served**, in green, and
**Refused**, in red, each scaled to its own highest bar (served requests
are many more). A bar is a button. It shows the events of that interval
and kind only (a served bar turns on Served alone, a refused bar the
refused kinds), a new query to the log store, not a filter of the rows
already loaded; the bar stays marked, the others fade, and **Served,
20:00–21:00 ✕** (or **Refused, …**) above the list takes you back to the
whole period. **Live** pauses (an
interval of the past does not change) and **Export CSV** exports what you
see. Field values are never stored, only the field name and the rule
that refused it. The IP address is the visitor's real one, the same the
legacy site receives in `X-Forwarded-For` (its own logs and bans keep
working).

![Logs](/help-img/logs.png)

### Activity

Who did what, in plain sentences: logins, invitations, maps written,
published and rolled back, discoveries (by you or by the agent), alert
emails with what they reported and who got them, exports. Search for a
person, a site, a connector or an action, and choose the period (last 24
hours to 90 days, all time, or from one day to another); **Show older**
goes further back.

![Activity](/help-img/activity.png)

## The assistant

Each site has an assistant: an AI that reads the site's protection and
answers your questions about it in plain words. Open it with **Ask AI** in
the site's header; it opens on the right of the page (over the whole
screen on a phone) and stays open while you move between the site's tabs.
It uses your own model, the one in **Settings → AI**: without one it says
so and does nothing else.

### Your conversations

- **Yours only.** Your conversations are kept for you: no other person of
  your organisation reads them in the panel, administrators included, nor
  the platform operator. They are stored with the rest of your data in
  the platform's database.
- **Ask AI** opens your latest conversation about the site you are on, so
  you go on where you stopped, also from another device.
- **History** lists all your conversations, on every site, newest first,
  with the site and the date. One of another site opens that site with
  the conversation. **Delete** removes one at once.
- **New** starts a clean conversation, for another subject: the AI no
  longer carries what was said before (a long conversation also makes it
  slower). The previous one stays in History.
- **How long**: an administrator chooses it in **Settings** ("Assistant",
  30 days by default, 1 to 365), counted from the last message. Lowering
  it deletes the older conversations at once.
- A proposed change shows, when you open the conversation again, whether
  it already became a draft and which one. A proposal can be made a draft
  for the same number of days after it was proposed; after that, ask
  again.

### What an answer looks like

- **Words**, formatted: short paragraphs, lists, **bold** for the key
  point, paths and values as `code`, a small table when it compares. It
  never shows images or links to other websites.
- **Cards** under the words, drawn by the panel from the real data (the
  AI only chooses which one, so the numbers and paths in a card are never
  the AI's):
  - **an event**: time, request ID, blocked or served with its status,
    the request, the rule that refused it, the visitor's code, and
    **Open in Logs**, which opens the Logs page on that request in a new
    tab (the conversation stays);
  - **the refusals of a period**: by reason, with requests and distinct
    visitors, to tell a scan (many requests, few visitors) from a gap
    (many visitors);
  - **an entry of the map**: its address and what it checks on each
    value (`?orderby` one of price, date; `{slug}` text, 80 characters at
    most), with a link to that map version.
- **Proposed changes**, when there is one (below).

### What it reads

Only the site you are on, and only what you could read yourself:

- **Its status**, as the **Status** page shows it: the connector online
  or not, the legacy application answering or failing, the certificate
  and when it expires, the isolation check, the last 7 days.
- **The versions of the map**: which is published, which drafts wait,
  who made and approved each one, and **what changed between two
  versions** with its risks (the same comparison as the review aid).
- **The check of a version** on the site: pages identical, inputs that
  refuse every attack, what breaks, what is still open, the agent's
  corrections.
- **The map**: the published version, or any version you name, with every
  entry, the checks on each value, the forms, the file folders and the
  site area.
- **The last discovery's report**: the CMS and its version, the plugins,
  themes and libraries with their versions, pages found, what is still
  open and why, what was kept out, what was added from visitors' requests.
- **The security scans** of the [Security tab](#security-scans): each
  finding, how severe, what the map does with it, and the draft built to
  harden it.
- **This guide**, to answer how to do something in the panel.
- **The logs** of the site, up to 30 days: what was served and refused,
  why, on which entry, how many different visitors.
- **The site's history** in Activity: discoveries, drafts, publications,
  alerts, who did what and when.

It looks these up before it answers, and says so when they do not tell.

### What you can ask

- **Whether the site works**: "Is everything working on this site?",
  "When does the certificate expire?", "Was the connector offline this
  week?". It reads the status and points out what needs attention.
- **What the site runs**: "Which WordPress version and which plugins?",
  "Is anything in the report outdated?". It gives the versions the
  discovery found and says when one looks old. It names a vulnerability
  only when the report lists it among the known issues or a security scan
  found it; it does not search for vulnerabilities (CVEs) itself and never
  invents one.
- **About the security scans**: "What is still open on this site?", "Is
  this finding serious?", "How do I close the robots.txt one?". It reads
  the Security tab: each finding, how severe, what the map does with it,
  whether the draft built after the scan hardens it (and where to review
  that draft), and what is left to fix on the server and how.
- **About the versions of the map**: "What changed in version 6?",
  "Which version is waiting for review, and is it safe?", "How did the
  check of version 5 go?".
- **How to do something**: "How do I check the pages behind the login?",
  "How do I upload the certificate?". It answers from this guide and says
  which page to open and what to press.
- **Why something was refused**: "Why was the request to /shop/ at 14:02
  blocked?", "What happened to request 9c41e0b2?". It finds the event,
  the rule that refused it and whether it looks like a visitor or a scan
  (how many visitors, how many requests, which entries and paths; the
  values themselves are never logged).
- **Whether real visitors are being refused**: "Did real visitors get
  refused today?", "Which refusals this week came from more than three
  visitors?". It sums the refusals by reason and by entry and points out
  the ones that look like a gap in the map rather than an attack.
- **What the map does**: "What does the map check on the home page?",
  "Which file types does /wp-content/uploads/ serve?", "Does version 6
  still check `?orderby=`?".
- **What a discovery found**: "What did the last discovery leave open, and
  why?", "Which pages were kept out?".
- **Who did what**: "Who published the current map, and when?", "When did
  the last discovery run, and who started it?".
- **How to tune it**: "Can the shop sort by date?", "PDFs in the uploads
  folder are refused: should they be?". When a change is worth making, it
  proposes it (below).

Ask in your language: it answers in the language of your question.

### Changes it proposes

The assistant never changes a map. When a change is worth making it shows
a **Proposed change** card: what the change does and why. An
administrator clicks **Create a draft and check it**: the panel makes a
new draft (on the latest draft if there is one, otherwise on the published
map) and the agent checks it on the site, as for any draft. While a
version is under review, or the latest draft is being checked, it waits:
decide that one first. Nothing
reaches visitors until an administrator reviews and publishes the draft
(see [Review and publication](#review-and-publication)). A viewer sees the
card and cannot make it a draft.

It can propose four kinds of change, and only these, each checked by the
panel before the card is shown and again when the draft is made (the new
map must be valid and accepted by the gateway):

- **A file type for a file folder** (`application/pdf` for
  `/wp-content/uploads/`). Types that can run code or hide what they are
  (scripts, HTML, PHP, executables, unknown binaries) are never proposed.
- **A new query key on a page** (`?lang=` on the shop), with a narrow
  check: one of the values the site offers, a whole number, a date, or
  short text without markup.
- **More values for a key that takes a list** (`date` for the shop's
  `?orderby=`).
- **A narrower check for a free-text field** of a page or a form
  (`?attribute_pa_color=` of a product becomes a slug of 40 characters, a
  review's `rating` a number from 1 to 5): a whole number, a date, an
  email address, one of a few values, yes or no, digits only, a slug, a
  key (letters, digits, `_ . -`), or shorter text. The patterns are
  Equalizer's own, never written by the AI; only a field that takes free
  text (no pattern, list or set of characters of its own) is narrowed,
  never widened: never longer than it takes now, still held to the map's
  default characters and to the values it refused, and required or
  optional as it was. The agent's check of the draft shows any page the narrower
  field would break, before you publish.

At most three proposals per answer. It does not propose new pages, forms,
logins, rules, rate limits or other settings, and does not offer to: it
says where in the panel they are changed by hand (a discovery, or
editing the map).

### What stays private

- Visitors' IP addresses never reach the AI: each visitor appears as a
  code (`V-3fa9c1`), the same for the same visitor across your questions;
  an address you type in a question becomes a code too. Email addresses
  in paths and questions are hidden.
- People of your organisation reach the AI as codes and are shown to you
  by their email.
- What comes from the site, its visitors or the logs is given to the AI
  as data, never as instructions: a path that says "ignore your rules"
  changes nothing.
- Activity records that someone asked the assistant about a site, which
  sources it read and how many changes it proposed, never the question or
  the answer. The conversation itself is kept only for you (above).
- Your question and what the assistant reads go to your AI provider, as
  for discovery. Its own terms say how long it keeps them.

### Limits

- 20 questions every 10 minutes per person; the AI is given the last 20
  messages of the conversation.
- A long answer the AI provider cuts at its length limit is completed:
  the assistant asks it to go on, up to twice. If it is still cut, it
  says so at the end; ask for the rest.
- It reads up to 500 events per question: on a busy site it says when it
  only saw the latest ones; the **Logs** page shows everything.
- It does not open pages of the site and does not see the legacy server.
- Without logs on the platform it answers from the map, the report and
  the history, and says the logs are not available.
- The AI can be wrong: check what matters in **Logs** or on the **Map**
  tab, and read a proposed change before you publish it.

## Security scans

The **Security** tab of a site scans it with your own vulnerability
scanner, CVE Radar, connected in [Settings → Security
scanner](#security-scanner), and reads every finding against the
published map.

### Two targets: through Equalizer and the original site

- **Through Equalizer**: the site's public domain, as visitors reach it.
  What a scan finds here is what is left open with Equalizer in front.
- **Original site**: the legacy site's own name, from the published map
  (for example `www.example.com` behind `test.example.org`), scanned
  without Equalizer, for a before and after. It is an attack on the site
  without protection: **Scan now** asks you to confirm that the site is
  yours or that its owner allowed it in writing, and Activity records who
  confirmed. It works only while the site still answers on its own name:
  after [isolation](#point-the-domain-and-isolate-the-site) the scanner
  cannot reach it. Only a site reached directly has one, and only when its
  name leads to public addresses: a site reached through the connector, or
  by address, has no original target, and an internal name is never
  handed to the scanner.

Both need a published map. Administrators scan; viewers read. **Scan now**
offers a **quick** scan (seconds: headers, HTTPS, cookies, email DNS,
technologies, exposed files), a **standard** one (a few minutes) or a
**deep** one (longer, more requests). One scan per target at a time; each counts against your CVE
Radar limits (20 scans a day). The tab follows the scan and shows the
findings when it ends; a scan that does not end within two hours fails,
and one the scanner never started (the panel stopped while asking it)
fails after five minutes. While its draft waits for the map to be free (a
version in review), the tab keeps checking, every minute, for a day.
The scanner creates its own record of the domain the first time, without
asking for its DNS check.

### What each finding says

Findings are listed by severity (critical, high, medium, low, info), with
the address, the method and the input the scanner names. Click one for its
evidence, CVSS, CVE and CWE. **With Equalizer** says what the published map
does with that address:

| Says | Meaning |
| --- | --- |
| Closed | the gateway refuses the request: nothing serves it (and no area), the entry does not take that method or input, or its own check refuses the value |
| Checked | a precise entry serves it and checks this input by its type |
| Free text | a precise entry serves it, but the input is free text: its length, markup and rules are checked, not its shape. Give it a pattern or a type if you can |
| Area only | only the generic area serves it, rebuilt but not checked precisely: no precise entry has the address, or the entry does not take this path value or query key. Add or widen a precise entry |
| File | a static file the map serves as it is |
| Server | about the server (open ports, TLS, headers): the map has no say |
| Closed now | (through Equalizer only) the scanner reached it, but the current map refuses it: the map changed since the scan. Scan again to confirm |

The verdict follows the gateway's own order (precise entries first, the
area for the rest) and is worked out when you open the tab, so a new map
changes it without a new scan.

On the card of each target, the findings by severity and how many the map
leaves open (through Equalizer) or would close (original site): the
number to compare before and after. The scanner's risk score (0 to 100)
is shown too. Findings older than a year are deleted, as the scanner does.

### The draft that hardens what it can

When a scan ends, of either target, Equalizer builds by itself **one map
draft** that hardens the inputs the map serves as **Free text** where a
finding was: it adds the text rule that refuses that kind of attack
(markup for cross-site scripting, SQL characters for SQL injection,
template sequences for template or expression injection), read from the
weakness the scanner names. A rule narrows an input, it does not prove the
attack impossible (an attack without markup, quotes or template sequences
still passes), so the panel says **hardened**, not closed; scan again
once it is published to see what is left. Code injection gets no rule: no
text rule stops it. The draft goes where every draft goes, in the **Map**
tab:

- on the latest draft if there is one (it takes its place, so there is
  still one draft to review), otherwise on the published map;
- when a version already waiting in the Map tab has those rules (built
  after an earlier scan), no new draft: the card points to that version;
- while a version is in review or a draft is being checked, Equalizer
  waits and tries again every few minutes, for a day;
- its note says it comes from the scan and what it hardens;
- the agent checks it on the site like any draft, so a rule that would
  break real pages shows in **Broken by this version** before you publish;
- it is yours to review and publish; nothing changes on the site before.

In the Security tab the card of the target links the version and says
what it hardens (or why there is none), and each finding it hardens says
**Hardened in draft N**, linked to it in the Map tab; a version replaced
or set aside since is no longer linked. **Area only** findings need a
precise entry, which a discovery adds; **Server** ones are fixed on the
server. The draft is made in the name of the person who started the scan,
while they are an administrator. Ask the assistant about any finding: it
reads the scans too (the 20 most severe of each target, never their
evidence, which may hold cookies or tokens).

## The source reader

![The Source tab](/help-img/source.png)

Optional. The **Source** tab of a site sets up a small program, the source
reader, that reads the site's code and tells Equalizer what it found, so
that a discovery sees what the links do not show and types fields from the
code instead of guessing.

### What it reads and what it sends

It runs next to the connector, never on the web server, and only calls
the panel: no port is opened. It reads only code files (`.php`, `.phtml`,
`.inc`, `.module`) and the few files that say where an application's entry
points are or what a component is: a theme's `style.css`, `composer.json`
and `composer.lock`, routing files (`*.routing.yml`, `routing.yml`,
`routes.xml`, `webapi.xml`), manifests (`extension.json`, `skin.json`,
`*.info.yml`, `*.info`, `install.json`). Up to 1 MB each; it studies up to
20 000 files and 200 MB of the site's own code, and copies at most
150 000 files and 1 GB over SFTP or SMB. It never reads uploads, caches,
version control, dependencies (`vendor/`, `node_modules/`: their versions
come from `composer.lock`), and never the files with credentials, by
name, on any site:

| Application | Never read |
| --- | --- |
| WordPress | `wp-config.php`, `wp-config-*.php` |
| Joomla | `configuration.php` |
| Drupal | `settings.php`, `settings.*.php` (`settings.local.php`) |
| Magento | `app/etc/env.php`, `local.xml` |
| PrestaShop | `app/config/parameters.php`, `parameters.yml`, `config/settings.inc.php` |
| OpenCart, Moodle, phpBB | `config.php` (OpenCart's `admin/config.php` too), `config_*.php` |
| TYPO3 | `LocalConfiguration.php`, `AdditionalConfiguration.php`, `config/system/settings.php`, `additional.php` |
| MediaWiki | `LocalSettings.php` |
| Any site | `.env*`, keys and certificates (`*.pem`, `*.key`, `*.p12`, `id_rsa*`…), `.htpasswd`, `auth.json`, and the usual names of a custom site's database settings: `config*.php`, `conf.php`, `db*.php`, `database*.php`, `connect*.php`, `conn*.php`, `settings*.php`, `secrets*.php`, `credentials*.php`, `parameters*.php` |

A file with another name that holds a password written in the code (a
`define` of a password, a variable or key named like one with a value, a
database connection with its password) is dropped as soon as it is
opened and nothing from it is used. Over SFTP and SMB the reader first
lists the folder and copies only the files it reads: the others never
cross the network.

It studies only the code that is the site's own: its extensions (plugins,
modules, components, themes) and its custom files. The application's core
and the libraries it ships give only their versions, and are not even
read: they are known code, and counting them buried the site's own entry
points under thousands of lines. It finds the application at the site's
root or in one of its folders (WordPress in `/blog`, Drupal in `/web`,
Moodle 5 in `/public`); when that folder is the web server's root (`web`,
`public`, `public_html`, `docroot`, `htdocs`, `html`, `www`), its name is
not in the addresses, and files outside it are not entry points (no
browser opens them).

| Application | Its core, version only | Entry points it reads in the extensions |
| --- | --- | --- |
| WordPress | `wp-admin`, `wp-includes`, the `wp-*.php` files | REST routes, AJAX and admin-post actions (`action=`), PHP files without the `ABSPATH` guard |
| Drupal 8 and later | `core/` | the routes of each module's `*.routing.yml` (`_access: 'TRUE'` is public; a parameter's requirement types it) |
| Drupal 7 | `includes`, `misc`, the root `modules` and `themes` | `hook_menu` paths of the modules under `sites/` |
| Joomla | its own extensions, as its `ExtensionHelper` lists them | each component at `index.php?option=com_…` (the administrator's too), the web services its plugins add to `api/index.php`, PHP files without the `_JEXEC` guard; `$input->getInt()`, `getCmd()`… type the fields |
| Magento 2 | `app/code/Magento`, `vendor/magento`, `lib`, `pub`… | a module's controllers at `/frontName/folder/action` (`etc/frontend/routes.xml`, the administration's under `/admin/`), its REST routes (`etc/webapi.xml`, `anonymous` is public) |
| PrestaShop | the core folders, the modules its `composer.json` ships, the administration folder (found by its content) | each module's front controllers at `index.php?fc=module&module=…&controller=…`, PHP files without the `_PS_VERSION_` guard |
| OpenCart | everything but the extensions (`extension/<name>`, OpenCart 3's `extension/` subfolders) | each extension's controller methods at `index.php?route=…` (`admin/index.php` for the administration's) |
| TYPO3 | `typo3/sysext`, `vendor/typo3` | eID scripts (`index.php?eID=…`), backend and AJAX routes under `/typo3/` |
| MediaWiki | everything but `extensions/` and `skins/` | special pages (`title=Special:…`), API modules (`api.php?action=…`, `list=`, `prop=`, `meta=`), REST routes (`rest.php/…`) |
| phpBB | everything but `ext/` (and `ext/phpbb`) | the pages of each extension's `config/routing.yml`, under `app.php/` |
| Moodle | the plugins its `lib/plugins.json` lists | the pages of the added plugins (files that load `config.php`), their AJAX services (`db/services.php`, `loginrequired => false` is public); `optional_param(…, PARAM_INT)` and the like type the fields |

A plugin installed with Composer into `vendor/` (TYPO3's extensions often,
Magento's modules sometimes) counts as a library: only its version.
Moodle sub-plugins inside a plugin Moodle ships, and OpenCart 3's own
extensions, are read as the site's (OpenCart 3 keeps them in the same
folders as the added ones).

In WordPress code, a REST namespace or an AJAX action kept in a constant,
a property or a variable of the same file (`self::route_namespace`,
`$this->namespace . '/' . $this->rest_base`) is resolved; a part the file
does not fix shows as `{name}`, and an action built at run time
(`'wp_ajax_' . $action` in a loop) as `action=*`, so the entry point is
still listed with its file and line.

A risky call counts when a request value reaches it on the same line, or
through a variable of the same file set from one
(`$id = $_GET['id'];` … `$wpdb->query( "… $id" )`), unless the value
was made a number, a yes or no, a key or an escaped SQL value first
(`absint`, `(int)`, `sanitize_key`, `esc_sql`…). Comment lines never
count. `filter_input()` types a field when its filter validates
(`FILTER_VALIDATE_INT`, `_EMAIL`, `_URL`, `_BOOL`); a `FILTER_SANITIZE_`
filter only strips characters, so the field stays untyped. A constant or
variable resolved into a route or an action that looks like a secret
stays a `{name}`. A key compared with a
value that looks like a token or a password (a backdoor key written in the
code) is never sent as a list of values: the value stays on your network.
Only regular files are read: a symlink, a FIFO or a device never is.

Elsewhere, a PHP file counts as an entry point only when it does
something as soon as it is opened: outside its named functions and
classes it reads the request (also inside a double-quoted string or a
heredoc) or makes a risky call. A closure handed to a hook
(`add_action('init', function () { … })`) counts: it runs on every
request. A file that only declares functions or classes, or a template
that prints what it is given, does not. This is read from the code's
structure (braces, strings, heredocs, comments, attributes), not with a
PHP parser: an approximation that can miss an unusual file. A backtick
counts as running a command only around a request value on the same line:
in a query it quotes a table or column name. It sends a summary: entry
points (with the query that selects them, as `option=com_shop`), the type
the code gives each request key, the calls where a request value may do
harm (file, line and the extension they belong to), the components and
their versions. Names, types and places only: **the code, the values and
the secrets never leave your network**. Files copied over SFTP or SMB are
read in a temporary folder deleted after each read.

### Set it up

1. **How the reader reaches the files** (administrators), always read only:
   - **SFTP (SSH)**: server, port, user and the site's folder. Make a user
     that can only read that folder. The reader makes its own key at its
     first start; the tab then shows it: add that line to the user's
     `~/.ssh/authorized_keys`. The server's key is learnt the first time
     and checked after.
   - **Windows share (SMB)**: server, share, user, password and the folder
     in the share. Use an account that can only read the share; the
     password is encrypted and never shown again, and serves only the same
     server and user.
   - **Folder on the same server (Docker too)**: the reader runs on the
     site's own server. Give the site's folder (for example
     `/var/www/html`): not the whole disk (`/`) nor a folder of the
     system (`/etc`, `/root`, `/usr`…), which the panel refuses. A path
     with spaces is fine: the command quotes it. A reader installed
     before 0.2.30 keeps reading the folder it mounted at `/site`. If the site runs in Docker, also give its
     **container** as `docker ps` names it (for example `wp`): the reader
     takes the container's volumes, read only (`--volumes-from wp:ro`).
     Without a container, the folder of the server is mounted read only at
     the same path. The command the panel gives already holds the mount:
     nothing to add by hand. A site whose code is only inside its image
     (no volume) cannot be read this way yet.
2. **Make token and command**, then run the command next to the
   connector (with a folder or a container, on the site's server). The
   command follows the settings above: change them, make a new token. The token is shown once; a new one stops the old one.
3. The tab says **Ready** once the reader answers (it calls every minute).
   **Read now** reads the files and shows what was found, or why it failed:
   it is also the test of the settings. A read no reader takes within 15
   minutes, or that runs for more than an hour (a copy gives up after 40
   minutes), fails with the reason, and a
   new token ends the reads of the old one: one read never holds the site
   for good.

**Remove the reader** forgets the settings, the token and the reads.
Viewers see the tab, not the settings form.

## Settings

### Alerts

**Settings → Alerts**: an email when something stays wrong for more than a
few minutes, and when a draft is ready. Choose which: connector offline,
legacy site failing, site reachable again on its old IP, certificate
missing or expired, certificate about to end, the site working again
after a problem, and **a discovery's draft ready to publish** — sent once
the agent has checked (and corrected) the draft, with what changes, what
came from visitors' requests, the agent's check and the link to the
draft; never when a discovery finds nothing to change. Nothing is
published until you do. It goes to the
administrators of your organisation and to up to five more addresses (an
on-call mailbox, a ticketing address). On by default; each change is sent
once.

### Assistant

**Settings → Assistant**: how many days each person's conversations with
the assistant are kept after their last message (30 by default, 1 to
365). Nobody reads another person's conversations; lowering the number
deletes the older ones at once. See [Your conversations](#your-conversations).

### AI model

**Settings → AI**: your own model (Claude, OpenAI or your own server) names
routes, spots sensitive pages and narrows types during discovery,
summarises versions before you publish them, and answers in each site's
[assistant](#the-assistant). Once the provider and the key
are in, the panel lists the models the key may use: pick one. The key is
yours, encrypted and never shown again. Without a model discovery still
works; the assistant needs one.

![Settings](/help-img/settings-ai.png)

### Security scanner

**Settings → Security scanner**: your CVE Radar account, for the
[Security tab](#security-scans) of each site. A card names the provider
and how the connection stands, with a small dot: **Connected** (the last
call to CVE Radar, saving the key, a scan or reading its results, was
accepted), **Not connected** (no key saved) or **Key refused** (CVE Radar
refused the saved key at that call: revoked, expired, or without the
permissions; save a new one). The panel never calls CVE Radar just to show
this page. Enter the address of its API
(from CVE Radar's documentation) and a key created in CVE Radar's API
section with the **read**, **scan** and **write** permissions (a
`cvr_test_…` key for trials). The panel tries them before saving, keeps the
key encrypted and shows only its last four characters; a new address needs
the key again. Keys expire after
90 days in CVE Radar: enter the new one here. **Disconnect** forgets the
key; past scans stay readable. The panel connects only to public
addresses on port 443, never to your network.

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
(405). A path not in the map has **Add to the map**. Or ask the site's
[assistant](#the-assistant): "why was /path refused?" — it finds the
event, explains the rule and, if real visitors need it, proposes the
change.

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
