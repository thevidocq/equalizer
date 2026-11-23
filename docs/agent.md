# The agent: studying a legacy site

The agent is what makes Equalizer practical. A gateway that only lets
through what the map declares is as good as its map: a map that is too wide
protects little, a map that is too narrow breaks the site. Writing a tight
and correct map by hand takes days of an expert's time per site, and nobody
does it. The agent does it: it studies the site, writes the map and proves
that the map works, so a person only has to review a result that is already
measured.

This document describes the agent as it is designed. The table in
[Rollout](#rollout) says which parts exist today.

## In one paragraph

The agent opens the site with a real browser, read-only, as a visitor and
with test accounts (administrator included). It learns what each page does,
which values each parameter and field takes, and which accounts use what. It
writes a map with a precise entry for every function it understood, and
leaves to the generic `/` area only what it did not, each with its reason.
Then it checks its own work twice, without ever writing to the legacy site:
every page is compared with and without Equalizer (the site must work the
same), and every parameter and field is tried with a set of attacks against
the gateway alone (the attacks must be refused). Where a check fails, it
widens or narrows that entry and checks again. The draft arrives in the panel
with a score that was measured, not estimated.

## Today and after the study

| | Today | With the study |
| --- | --- | --- |
| Exploration | Follows links and GET forms with plain HTTP requests | A real browser (headless Chrome): scripts run, their AJAX and REST calls are seen |
| Administration | Never opened | Opened read-only with a test administrator (Drupal, Joomla; WordPress writes when its screens are viewed, so it stays closed) |
| Accounts | One optional test account | Up to three (for example customer and administrator), each in its own pass, to reach the pages behind their login |
| Forms | Field names; hidden fields kept on the gateway | Also `type`, `maxlength`, `min`/`max`, `pattern`, `required`, the options of every `<select>` and radio group: tight types without guessing |
| APIs the site declares | Not read | Read: `/wp-json/` (WordPress lists its routes with argument types), Drupal JSON:API, OpenAPI files |
| Map | Precise entries for public pages, everything else in the `/` area | A precise entry for every function understood; the `/` area keeps only what is listed under "still open" |
| Checks | None: a broken page is found by using the site | Every page compared with and without Equalizer; every field tried with the attack set |
| Result | A draft | A draft with a score and the list of what is still open |
| AI | Reviews the structure (masked samples): rename, exclude, narrow | Unchanged (see [AI](#ai)) |

## Web application profiles

What the agent knows about a kind of application lives in one file per
application in `agent/equalizer_agent/webapps/` (WordPress, Joomla,
Drupal, Magento, PrestaShop, OpenCart, TYPO3, Moodle, MediaWiki, phpBB,
and `generic.py` for custom sites). A profile says how to recognise the
application (generator tag, paths, extensions), where a test account logs
in, where its administration is and whether the agent may open it, and
which of its pages act when they are only opened. The rules every site
shares (action links, one-time tokens, sensitive files) are in
`webapps/base.py`, once. The crawler, the map building and the
verification never name an application: they ask the profile of the one
recognised, or the generic one.

A profile opens an administration only when the lab proved that viewing it
changes nothing; until then it stays closed. The
[compatibility matrix](compatibility.md) is generated from the profiles
(`python3 -m equalizer_agent.webapps.matrix`) and the agent's tests fail
when it is stale, so it never says more than the code does.

## How a study runs

### 1. Explore

- **Seeds**: `/`, `robots.txt`, `sitemap.xml`, the template's pages, the
  pages at least two distinct visitors asked for (reported by the gateway;
  never a file, a query key or a secret-looking path), the APIs the site
  declares. Every XML answer that is a sitemap or a sitemap
  index is read, whatever its name (`/wp-sitemap.xml`, those `robots.txt`
  lists). Links and sitemap entries written with another name of the site
  (its public domain, its aliases, the `www.` / bare twin of each) are the
  site's own, and are fetched by their path like the others; the browser
  pass follows the same names.
- **Browser**: headless Chromium (Playwright) inside the agent opens the
  pages each pass found (up to 40 per pass), so their scripts run. Every
  request it makes is intercepted and answered by the agent's own fetcher,
  through the connector like every other request, only when it is a `GET`
  or `HEAD` for the site itself that the crawler's rules allow; everything
  else is refused and listed in the report ("kept in"). The browser's
  network otherwise points at a closed port, so nothing the interception
  does not cover (a WebSocket) gets out. It never clicks, types or submits.
  Requests the scripts make become pages to study like any other; a script
  request answered with JSON stays with the site area and is listed as
  still open (map APIs need their shapes). Without Playwright
  (`EQ_BROWSER=off`) the study runs with plain requests.
- **Files**: every file a page loads becomes an asset of the map, with
  its type from the file extension: `src`, `data-src` and other lazy-loading
  copies, each `srcset` candidate, a video's `poster`, `<style>` blocks and
  `style=""` attributes, and the stylesheets themselves (up to 60 per
  study, `@import` followed) for their fonts and background images. A
  prefix already in the map gets the new types, **passive types only**
  (`equalizer_agent/assettypes.py`: images, fonts, audio, video,
  stylesheets, PDF and office documents; scripts only when a page loads
  them): a type opens every file of that type under the prefix, linked or
  not, so text, CSV, archives, SQL, JSON, HTML and XML are never added by
  the agent. What visitors ask for is never evidence (see [Visitor
  requests](#visitor-requests)). `srcset` is split as the HTML standard says, so a
  CDN URL with commas stays whole. Documents (`.pdf`, `.docx`, `.xlsx`,
  `.odt`, `.csv`, …) are files too; archives, logs and dumps never are
  (they are not studied at all). No asset prefix is proposed over a folder
  where the study saw pages, forms or script requests: an asset prefix
  takes everything under it that no route matches, so those folders stay
  with the site area, files included.
- **Links**: `<a>`, `<area>`, `<iframe>`, `<frame>` and
  `<meta http-equiv="refresh">` targets; `<link rel="manifest">` and
  `<input type="image">` are files.
- **Passes**: one anonymous, then one per test account. Each pass records
  the pages, the requests the scripts make (with their parameters), the
  forms and their fields, and the links.
- **Limits**: `max_pages`, at most 20 URLs per shape (calendars, searches),
  `EQ_RATE` requests per second, as today.

### Visitor requests

The gateway reports what it refused visitors: paths not in the map, files
an asset prefix refused, query keys a route dropped (the route's pattern
and the key's name, never a value). Each report counts **distinct
visitors**: the gateway turns each client address into a short key (an
HMAC of the path and the address with a secret only the gateway holds,
its form key, 8 bytes) and sends up to 16 keys per path, never an
address; the panel, which does not know that secret, cannot turn a key
back into an address nor link one visitor's requests across paths. It
keeps up to 16 keys per path, so one visitor coming back every minute is
still one visitor. Counts recorded before 0.2.11 summed every gateway
batch: they count as one visitor until new keys arrive. The job carries
the 200 refused paths with the most visitors (last 30 days).

A visitor's request is never evidence on its own: anyone can ask for any
address, and a public site gets scanners all day (`/CFIDE/administrator`,
`/crossdomain.xml`, `/.env`). So the agent adds one to its draft only
after checking it on the site, and only in a form where nothing a visitor
sends decides what passes (`equalizer_agent/visitors.py`). What it adds is
listed in the report (`visitors_added`) with its reason, and in the email
that says the draft is ready, so whoever publishes sees it and can take
it out first. Nothing else from visitors reaches the draft or the report.

- **A file** is added when all of these hold: at least **2 distinct
  visitors** asked for it; it is under an asset prefix of the map; its
  type (by extension, then as the site sends it) is **passive**
  (`assettypes.py`: images, fonts, audio, video, stylesheets, PDF and
  office documents), so a `.php`, a script or a page never is, however
  many ask; no page of the site names it (the study adds those itself);
  and the agent, opening it anonymously on the site, gets a 200 **of that
  kind** (HEAD; the file itself when HEAD is not answered with it, such
  as 405 or a firewall's 403; a font always, judged by its first bytes as
  the gateway does, since servers send fonts as
  `application/octet-stream`). The type added to the prefix is **the one
  the site sends** (`image/vnd.microsoft.icon` for a `.ico`, `audio/x-wav`
  for a `.wav`), since that is what the gateway compares; a type the
  prefix already lists is not added again (the file is then refused for
  its size or content). A path with spaces or accents is opened encoded,
  as a browser sends it. A scanner's guess does not exist (404). Names
  that are actions or sensitive files (`rules.py`) are never opened. Up to
  20 files are opened per discovery. Adding a type serves every file of
  that type under the prefix: a leftover file the site already serves to
  anyone is the legacy server's concern, as everywhere else in Equalizer.
- **A query key** is added when: at least **2 distinct visitors** used it
  on a GET route whose legacy request is a GET; the map can take it there
  (a name, not a path parameter, not declared, not a key the site already
  gets under that name); it is a **presentation key**
  (`visitors.PRESENTATION`: a sort, an order, a page number, a page size,
  a view, a language, a currency, a category or a shop filter such as
  `filter_*`, `pa_*`, `min_price`), because the agent opens the page with
  it and a GET can act (`?add-to-cart=`, `?doing_wp_cron=`): any other
  key is never opened nor added; **the site offers its values**; and **the
  page shows other content with one of them**.
  - **Values come from the site, never from visitors** (the gateway does
    not even send them). A page number (`page`, `paged`, `pg`) is an
    integer from 1 to 100000, unless the site offers other values for it
    (`?page=contact`: then it is not added). A bound (`min_*`, `max_*`) is
    an integer from 0 only when the site offers whole numbers for it (a
    decimal price or a date: not added). Any other key is an `enum` of the
    values the site itself offers: the options of its `<select
    name="key">` and the values of its links to pages of the same route,
    on the site's own host (at most 50, each a plain token
    `[A-Za-z0-9_.,:-]`, so no spaces, quotes, brackets or SQL even in a
    link a comment wrote). The choices must be **the same on every page**
    read that offers them, and must include every value the study saw in
    the site's links for that route: a value missing from the map would
    refuse a real visitor's page (the gateway refuses a value outside the
    `enum`, an attack included: `?orderby=1 AND SLEEP(5)`). A site-wide
    choice (`visitors.SITE_WIDE`: a sort, a view, a language, a currency)
    may come from one page; a filter's choices (`filter_*`, a colour, a
    category) change with the page, so they must be the same on two pages
    at least, unless the route is one page. When the site offers no values
    the key is not added: a value would then be free text.
  - **The page must change.** A pattern such as `/{slug}/` covers the shop
    and the cart alike, so the agent reads up to 6 studied pages of the
    route and tries the key on up to 3, those that offer it themselves
    first (WooCommerce's `<select name="orderby">`). On each it loads the
    page and the page with a key no site knows (`?eqprobe=…`): a page cache
    serves the plain page and builds any page with a query, so the noise
    (dates, counters, rotating posts) is measured between the two, and the
    page with the key (one of the offered values, up to 3) is compared
    with the built one. Every answer must be the page itself (200, HTML):
    a redirect or an error is not a page option (`?author=1` redirects to
    the author's page, a firewall answers 403). Without noise any other
    word is a change (a sort only reorders); with noise the text must move
    well beyond it (twice it, and at least `1 - TEXT_SIMILAR`). A key the
    site ignores (`?test=1`) changes nothing and is left out; a page that
    changes on every load tells nothing. Action-like and excluded URLs
    (`rules.py`) are never opened. Up to 10 keys are tried per discovery.

Everything is opened anonymously: a test account's session is set aside
meanwhile. The check runs after the AI review, on the draft's final
routes. What is added makes the discovery's draft differ from the
published map, so a continuous discovery proposes a draft for it; the
draft is verified and corrected like any other, and when it is ready the
tenant's administrators get the **draft ready** email (`draft_ready` alert
kind, Settings → Alerts). Reports of agents before 0.2.11 listed requests
unchecked (`files_refused`, `query_dropped`, with a `verdict`), and those
of 0.2.11 offered a button to add them: the panel shows neither any more.

### 2. Understand

Deterministic, no AI needed:

- **Routes**: varying path segments become typed parameters (`{id}` integer,
  `{slug}`), as today. A route already in the map gets the query keys the
  site's own links use and it lacked (the route the gateway would pick:
  `/search` before `/{slug}`, and only one whose values fit their types,
  as the gateway checks: `/notice.php?id=1` is not a page of a `/{slug}`
  whose type refuses dots, so it gets a route of its own instead of giving
  its key to one that never serves it; a GET route whose legacy request is
  a GET; keys a map can hold; only keys seen with a value), typed from
  those values as for a new route (`query_added` in the report): a request
  with a key its route does not declare goes to the area, where its
  values are not checked. The gateway also reports the names of those
  keys for visitors, with the route's pattern (`/permits/{id}?orderby=`;
  never a value, nor tracking keys such as `utm_*`, `fbclid`, `gclid`):
  added only once checked on the site (see [Visitor
  requests](#visitor-requests)).
- **Values**: every parameter gets the narrowest type that fits all the
  values seen and the form's own attributes: an integer with its range, a
  list of values (`enum`) when the site offers a closed set (a `<select>`,
  links that only ever use `asc` and `desc`), a date, an email, a slug, a
  text with its maximum length.
- **Form fields** (read-only, so from what the page declares, never from
  submitted values): a `<select>`, radio group or checkbox becomes a list of
  values; a number its range; email, date, phone, URL, colour and time
  their formats; an HTML `pattern` is kept when it means the same in the
  gateway's RE2 (no lookaround, at most 1000 repeats).
- **Text fields**: every free-text field refuses HTML and template code
  (`no_html`, `no_template_meta`). A single-line field whose name says it
  holds a name or a code (name, city, zip, phone, code…) also refuses SQL
  delimiters (`no_sql_meta`, an apostrophe between letters stays allowed).
  The others (a search, a message, a nickname) must still accept them and
  are listed under "still open" with the reason.
- **Access stays with the site**: the map does not restrict pages by
  account. Equalizer rebuilds every request and the legacy site keeps
  deciding who sees what (a project decision). The accounts only let the
  agent reach and study the pages behind a login; the report says which
  account reached what.
- **How it logs in**: like a person, with the site's own login form. Of the
  forms with a password field it takes a login page's first (and one with a
  single password before a sign-up form), reopens the page that shows it
  in the account's own session (tokens in the form or in its address, such
  as OpenCart's, belong to that session), and posts it once. The login
  worked when the site starts a new session and stops asking for the
  password; a site that keeps its session cookie is asked the login page
  again, which it no longer shows. A login made in script that answers in
  JSON (OpenCart) is judged by that answer: a redirect to a page of the site
  other than the login is a success, anything else a failure, whatever
  cookies came with it. Where the site's visitors
  have no login (TYPO3 out of the box), the report says `no_login_form`.

### 3. Write the map

Precise entries for every function understood. The `/` area stays in the
draft, but every path that still falls into it is listed with its reason
(not reached, answers an error, an action never performed read-only, a value
that cannot be narrowed). The share of the site that still falls into the
area is part of the score.

### 4. Check that the site still works

For every page and every script request seen in step 1:

1. the agent opens it directly through the connector, as in step 1;
2. it asks the gateway to serve it with the **draft** map (preview, below);
3. it compares the two: status, visible text, links and forms (after the
   address rewrite), and a screenshot of both in the browser.

Forms are never submitted. For each form the agent builds the request the
browser would send with the values it saw and asks the gateway, in dry-run,
whether it would accept it.

A difference means the draft is too narrow there: the agent widens that
entry (or gives it back to the area) and checks again.

**Files.** A page can look the same while its fonts, images or videos are
refused. Every file the public pages name (in their HTML, their links and
their stylesheets, up to 200 files and 30 stylesheets) that an asset prefix
of the draft serves is opened directly and asked through the gateway
(preview with `HEAD`: the verdict, not the body). Sensitive files and action
links are never opened, as in the study (a file is judged by its folders
and only against the strong words — logout, delete, unsubscribe…: `reset.css`
and `/lib/dropzone/…js` are checked, `/account/logout/bye.png` is not
opened); a page answering under a prefix
is left to the page comparison. A file the site serves and the gateway
refuses is a broken page; the correction adds the type the site sends if
it is passive (as in discovery: scripts only when a page loads them), or
the `max_size_kb` the file needs (size plus 10%, up to 4 GiB; a prefix
with videos never goes below their 512 MiB). A prefix is never removed:
a page it refuses stays broken, for the person to decide.

### 5. Check that attacks are refused

For every parameter and field of the draft, the agent asks the gateway, in
dry-run, whether it would accept each value of a fixed attack set:

- SQL (`' OR 1=1--`, `1;DROP`, comments, stacked queries);
- markup and script (`<script>`, event handlers, `javascript:`);
- template and server code (`{{7*7}}`, `${…}`, `<?php`);
- paths (`../`, encoded variants, null bytes);
- shell (`;`, `|`, `$(…)`, backticks);
- out of type (letters in a number, out-of-range values, oversized values,
  control characters).

A value that would be accepted means the entry is too wide: the agent
narrows it when the values it saw allow it, and checks the site again (step
4). When it cannot narrow without breaking the site, the field is listed
under "still open" with the reason (for example: the post editor accepts
HTML by design).

Nothing in this step reaches the legacy site: a dry-run stops at the gateway.

### 6. Hand over

The draft is saved like any draft, with the report:

- **score**: functions identical through Equalizer (n of total), fields and
  parameters that refuse the whole attack set (n of total), share of the
  site left to the area;
- **what the site does**: each function with its pages, the accounts that
  use it and the rule written for it;
- **still open**: every path left to the area and every field that accepts
  part of the attack set, each with its reason.

A person reviews it, may change it in the editor, presses **Check this version on the site** to run
steps 4 and 5 again on their version, and approves. Nothing is ever applied
without approval.

### What a verification measures today

- **Pages**: the pages the latest discovery studied anonymously (the
  version's own literal routes when there is none), and those it studied
  with each test account, behind their login: a discovery hands its
  accounts, still sealed, to the verification of its draft, which erases
  them when it ends; Verify by hand takes accounts again. The agent logs
  each account in directly, opens a try session on the gateway with the
  account's legacy cookies, and compares each page both ways with that
  session. A page is identical when the status (or, for a redirect, where
  it goes), the visible text (97% of the words, as dates and counters
  change between two loads; decoded in each answer's own charset; the
  site's own address counts the same whether written as the legacy host or
  the public one), the internal links (relative ones resolved against the
  page) and the number of forms are the same. A text difference names a
  few of the words that differ.
- **Inputs**: every query parameter and path parameter of a route that a
  studied page shows how to reach, and every field of a form found on a
  studied page (a valid submission first; if the version refuses it, the
  form is broken). Each gets 20 values in six groups (SQL, HTML and script,
  template and server code, paths, shell, out of type). A value that falls
  out of a route goes to the site area: if the area serves it, the input
  is not protected, which is how the score shows what the area lets
  through.
- **Correcting the draft** (`correct.py`), for drafts a discovery proposes
  and when asked on Verify: up to three rounds. Widen: a field the gateway
  refused on a studied page becomes plain text (no HTML, no template code);
  an entry that breaks a page for another reason goes back to the site
  area; kept only if more pages work. Narrow: a query parameter an attack
  got through becomes a number when every value seen (at least three) is
  one, or letters, digits and `. , _ -` when they all are; kept only if no
  page breaks and more inputs refuse every attack. The panel signs each
  corrected version for tries (`/internal/verifications/{id}/try-map`, the
  same checks as any draft); the result is a new draft with the changes
  and their reasons, carrying its own measure and the corrections. After a
  discovery the person reviews one draft: the panel retires the first one
  (unsigned, never published, so never a rollback target) and the
  discovery proposes the corrected version (`Store.Supersede`, audit
  `map.superseded`). A verification asked by a person keeps both versions,
and a draft a person has submitted, approved or published over meanwhile
is never corrected nor superseded: its measure is kept (a correction made
while the person took the draft on is withdrawn, its version retired).

In the lab, with the editor as test administrator: pages identical
12/12 (WordPress), 155/155 (Joomla, administration included), 154/154
(Drupal, administration included), 4/4 (intranet). Verifying behind the
logins first showed hundreds of differences; they were the comparison's
own (relative links, the site's address in the text), fixed above.

The first verification in the lab found a real bug: on a site served in
plain http, the gateway checked a form's token against the https binding
cookie, so every mapped form submission was refused. Fixed with a
regression test.

## Read-only, guaranteed

The agent never changes the legacy site. Three layers:

1. **No writes leave the agent.** Every request goes through one place
   (`fetch.py`; the browser's requests are answered by it too),
   which refuses every method other than `GET` and `HEAD`, except the login
   of the test accounts (a `POST` to the login form, and to the
   administration's own login where it has one, such as Joomla's). Forms
   are read, never submitted.
2. **No action links, no pages that act when viewed.** Some links change
   something with a plain `GET` (WordPress's "Trash", "Deactivate", "Log
   out"; Joomla checks an article out when its edit form opens). They are
   never opened: generic patterns (verbs such as `delete`, `trash`,
   `publish`, `deactivate`, `logout` as whole words, `action=…`, `task=…`,
   `layout=edit`, links carrying a one-time token such as `_wpnonce` or
   Joomla's form token) and a fixed list of pages that run something when
   opened (scheduled jobs, installers, updates, exports, "add new" screens
   that save a draft). They are listed in the report. In the path, a
   segment that names content (a title of four words or more joined by `-`
   or `_`, with at most one such verb and never at either end:
   `how-to-remove-stains`; not `item-12-remove` nor `my-plan-cancel`) is a
   page, unless it holds `logout`, `delete`, `unsubscribe`, `destroy`,
   `purge` or `impersonate`; the query is always read in full. Ending the
   session is judged apart (`ends_session`), on each piece of the URL
   decoded, folded (NFKC, case, accents) and without separators, so no
   spelling escapes (`/WebLogOff.aspx`, `/Account/DoLog_Off`, `/log%2Dout`,
   `/log.out`, `ſignoff`): log out, log off, sign out, sign off, log me
   out, end session (`/connect/endsession`, `/session/end`), disconnect,
   and the usual words in German, Dutch, Swedish, French, Spanish,
   Portuguese, Italian, Polish and Czech, anywhere, except log/sign off
   followed by offer(s), offset, office or offline (catalog-offers,
   analog-offset, design-office, blog-offline). A file name is read without
   its extension too (`/logout.png` is the logout route on Rails), in the
   browser pass and in verification. A page that only talks about signing off
   is not studied: safer than a test account logged out. The cases the
   reviews found are kept as a regression corpus in
   `agent/tests/test_actions.py`.
3. **Checks stop at the gateway.** Preview only serves `GET` and `HEAD`;
   everything else is dry-run and never forwarded.

**Proven in the lab** (`lab/e2e/tests/test_06_study.py`): the agent
studies the ten lab sites (WordPress, Joomla, Drupal, a custom intranet,
OpenCart, PrestaShop, phpBB, MediaWiki, TYPO3, Magento) with a test account
(a customer on the shops) and its browser, and the content of each site's
database (pages, articles, products, posts, users, extensions,
configuration) is the same before and after. What each profile proves is in
[`compatibility.md`](compatibility.md). Drupal's and Joomla's
administrations are studied this way (Joomla's with its own login).

**WordPress's administration stays closed to the agent**, even with an
administrator account: the lab found that WordPress writes to its database
when its screens are only viewed (the dashboard saves a "Quick Draft", the
theme screens save global styles, the privacy screens save suggested
texts). Its public site and login are studied as before; its
administration stays served by the site area.

What no rule can catch: a custom application that changes data on a plain
link with nothing in its address to say so. For such sites, give the agent a
test account with the fewest permissions possible, or run the study against
a copy of the site.

### Test accounts with few permissions

Fewer permissions mean fewer pages studied (the rest stays in the area), but
a second safety belt. Per CMS:

- **Joomla**: a user group with "Admin Login" and "Access Administration
  Interface" on the components to study, and no Create, Delete, Edit, Edit
  State or Edit Own. This is close to a true read-only administrator.
- **Drupal**: a role with "Use the administration pages and help", "View the
  administration theme" and the "view"/"access … overview" permissions of
  each module. Permissions named "Administer …" also allow changes: leave
  them out, and those pages stay in the area.
- **WordPress**: there is no read-only administrator. A custom role (with a
  role plugin such as Members) can see most screens without editing, but
  settings pages require `manage_options`, which also saves them. Use the
  Editor role for content, or an administrator only on a copy of the site.
- **Custom applications**: depends on the application.

## AI

The study above is deterministic: it does not need an AI model. The tenant's
model keeps its current job: it reviews the structure (paths, parameter
names, masked samples), names entries, excludes what looks like a backup or
an installer, and narrows a type when every observed value fits. It never
opens anything, and its answer is checked field by field.

**Planned, not decided**: letting the model read page contents (text, forms,
scripts) so it understands what each page does and what each field holds,
which gives tighter maps. It would be off by default, enabled by the tenant,
with personal data masked before anything reaches the model, and preferably
with a model on the tenant's own network. Until it is decided, the agent
never sends page contents to a model.

## Contracts

- `schema/api/panel-api.yaml`:
  - `DiscoveryStart.accounts` (up to three test accounts with a label);
  - `DiscoveryJob.accounts`;
  - `StudyReport` (the `report` of a discovery: score, functions, still
    open);
  - `POST /apps/{app}/maps/{version}/verify` and `MapVersion.verification`
    (`Verification`): the **Check this version on the site** button;
  - the internal endpoints the agent uses to take and finish a verification.
- `schema/api/gateway-admin.yaml`: the gateway's admin endpoint (internal
  network only), now documented, with the new `POST /maps/try`:
  - `dry_run`: would the gateway accept this request with the draft, and if
    not which field and why; nothing is forwarded;
  - `preview` (`GET` and `HEAD` only): the gateway serves the request with
    the draft, through the connector, and returns the rebuilt answer;
  - the draft must be signed by panel-api for this purpose with a short
    expiry, and the call needs the service token, so only a version the
    panel handed to the agent can be tried, and only towards its own
    tenant's connector;
  - for test accounts, the agent logs in to the legacy site itself and
    hands the legacy cookies over; the gateway opens a try session with
    them and the account's role, and returns its cookie for the next tries;
  - tries use no rate limits (the attack set measures the map, not the
    limits), their events stay in the answer (an attack set never reaches
    the security logs), and a dry-run never changes a session.

## Rollout

Each step is its own pull request, tested in the lab before the next.

| Step | What | Status |
| --- | --- | --- |
| 1 | This document and the contracts | Done |
| 2 | Gateway: `POST /maps/try` (preview and dry-run); panel-api: signing for tries | Done |
| 3a | Agent: several accounts, form fields typed from their HTML, text rules, "still open" in the report; panel: accounts | Done |
| 3b | Agent: administration read-only (Drupal, Joomla; not WordPress), single read-only guard, a lab study of the four sites proving nothing changes | Done |
| 3c | Agent: browser (Playwright) behind the read-only guard, script requests; Chromium in the agent image | Done |
| 4 | Agent: the two checks and the score; panel-api verification queue (every draft a discovery proposes is verified), pages behind test accounts' logins, corrections into a new draft | Done |
| 5 | Panel: score, broken pages and "still open" on the version and in the discovery report, Verify button | Done |
| 6 | Lab: studies of WordPress, Joomla, Drupal and the intranet with the expected scores | Planned |

## Mockups

How the panel will show a study (fictitious company Acme; not built yet).

The Discovery tab after a study: the score, what the site does with the rule
written for each function, and what is still open with its reason.

![Discovery tab after a study](screenshots/mockups/study-discovery.png)

A draft changed by hand and verified again: the change broke one function,
and the panel says which and why.

![Verify after a manual change](screenshots/mockups/study-verify.png)
