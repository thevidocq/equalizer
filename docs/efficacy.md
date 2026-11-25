# Efficacy

How much of what Equalizer is meant to stop it stops, measured on real
sites: the lab's applications, with maps made by the real discovery agent,
served by the gateway of the release. The legacy site logs every request
it receives, so "reached the site" is measured, never assumed.

Run it with `python3 lab/e2e/efficacy/efficacy.py [site…]` (lab README);
it exits 1 when a guarantee below breaks. Run it before every release, on
every site the lab can build.

## Guarantees

| Guarantee | Checked as |
| --- | --- |
| The site is the same through Equalizer | every studied page: same status, same visible text (≥ 97% the same words, as the agent's verification) |
| An attack on a typed value never reaches the site | every payload in an integer, `enum`, date, boolean or email field is refused |
| An extra key does not carry it past | the same, with an undeclared key added (`&zz=1`) |
| No attack on the protocol reaches the site as sent | traversal (4 forms), null byte, TRACE, a 64 KB header, another host, request smuggling, XXE, PHP sent as an image, unknown binary bodies, forged method and identity headers |

## Method

- **Maps**: the agent studies each site from the kind's template, with the
  test account, as the panel would ask it (no AI, no browser), with the
  site's code read as the source reader reads it (`source-summary`; from
  0.2.35), and its draft is published as it is.
- **Parity**: the same measures for every kind of site, side by side: pages
  found, pages served by a precise entry, inputs of the map and how many
  are typed, what the code added, what the AI security study would be
  given. `readers.py` reads each kind of site with one real, widely used
  extension added, to show the reader goes as deep in every
  application's extensions as in WordPress plugins.
- **Visits**: every page the agent found anonymously, seen directly and
  through the gateway.
- **Attacks**: 8 families (SQL injection, XSS, path traversal, command
  injection, template injection, serialized PHP objects, CRLF, oversized
  values), 20 payloads, sent to every input point the map knows: each
  declared query key of each GET route, the same key with an extra
  undeclared key, each path parameter, each field of each form a visitor
  can open (on the page that shows it, with its token), a key no route
  declares, and a few paths only the area serves. Each request comes from
  its own address, so rate limits are not what is measured. A form field
  is measured only when a plain value reaches the site.
- **Verdicts**: *blocked* (refused, nothing reached the site), *dropped*
  (served without the value), *neutralised* (the value arrived without its
  dangerous part), *passed* (it arrived as sent). *Passed* is not
  *exploited*: the application has its own defences; this measures only
  what Equalizer lets through.

## Results (0.2.35): every kind of site the lab builds here

Seven sites: WordPress 6.9, Joomla 5.4, Drupal 11.4, the custom intranet
(generic profile), phpBB 3.3, MediaWiki 1.43, TYPO3 13.4. Joomla is built
without npm (its media from the release's full package) and TYPO3 runs on
PostgreSQL, so both are studied here now. OpenCart, PrestaShop and Magento
need MariaDB (Magento also PHP's `bcmath` and `soap`), which this machine
does not have: they are studied where the full lab runs. The lab sites
were made larger so that each kind is measured on a real surface: Joomla
sixteen articles, Drupal twenty articles with tags, comments and a
contact form, the intranet a staff directory, a calendar, a feedback form
and a JSON API, MediaWiki a handbook of twelve linked pages, TYPO3 eleven
pages.

| Measure | 0.2.35 (7 sites) |
| --- | --- |
| Pages the same as on the site | 394 of 394 |
| Pages served by a precise entry | 391 of 394 (99%) |
| Attacks on typed values blocked | 940 of 940 |
| The same with an extra key (`&zz=1`) blocked | 880 of 880 |
| Attacks on path parameters blocked | 160 of 240 (9 neutralised) |
| Attacks on text with a pattern blocked | 56 of 100 |
| Attacks on free text blocked | 949 of 1 940 |
| The same with an extra key blocked | 283 of 620 |
| Attacks in a key no entry declares | 0 of 1 040: the area's |
| Attacks on paths only the area serves | 0 of 380: the area's |
| Attacks on the protocol (13 per site) | 78 blocked, 13 neutralised, 0 through |

Parity (what the agent made of each site):

| Site | Pages | Precise | Inputs | Typed | Pattern | Free | Code files | Typed by code | Code-only entries | Risky calls | Components | Study evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| WordPress | 42 | 94/95 | 20 | 10 | 8 | 2 | 601 | 0 | 0 | 0 | 1 | C 17, E 12, O 55, X 80 |
| Joomla | 22 | 26/26 | 75 | 46 | 6 | 23 | 46 | 0 | 10 | 0 | 0 | C 96, E 25, O 42, X 80 |
| Drupal | 163 | 31/31 | 160 | 31 | 13 | 116 | 66 | 0 | 0 | 0 | 4 | C 72, E 101, O 148, X 9 |
| Generic | 8 | 42/42 | 15 | 6 | 3 | 6 | 13 | 0 | 3 | 0 | 0 | E 16, O 6, X 10 |
| phpBB | 10 | 46/47 | 47 | 22 | 3 | 22 | 2 | 0 | 0 | 0 | 1 | C 42, E 16, O 27, X 8 |
| MediaWiki | 18 | 141/142 | 21 | 6 | 3 | 12 | 73 | 0 | 0 | 0 | 0 | C 2, E 19, O 107, X 80 |
| TYPO3 | 11 | 11/11 | 3 | 0 | 3 | 0 | 3 | 0 | 0 | 0 | 0 | C 76, E 4, O 2 |

Reading: every kind of site is served the same, almost entirely by
precise entries, and every typed value is closed on each; the guarantees
hold on all seven. The free fields are the forms that must take text
(Drupal's comments and contact form are most of its 116). The lab sites
are stock installations: their core is known code, so few files are read
and no code-only entry or risky call appears; the reader's depth on
extensions is measured below. Study evidence counts what the AI would be
given (E entries, C components, O still open, X kept out).

### Source code: the same depth on every application

Until 0.2.34 the reader typed fields from `$_GET` and the like, Joomla's
input and Moodle's `optional_param`: on Drupal, MediaWiki, phpBB, TYPO3,
OpenCart, PrestaShop and Magento extensions it found entry points but
almost no inputs, and it skipped the phpBB team's own extensions
(`ext/phpbb/boardrules`) as core. 0.2.35 reads each application's request
reader (Symfony's `query->getInt()`, MediaWiki's `getInt()`/`getBool()`,
phpBB's `variable('x', 0)`, Extbase's `getArgument()`, OpenCart's
`request->get[…]`, `Tools::getValue()`, `getParam()`), with `(int)` in
front typing it, and follows them into risky calls as it follows
`$_GET`. `readers.py`, one real extension per kind:

| Site | Extension | Stack | Files read | Entry points | Inputs | Types | Risky calls |
| --- | --- | --- | --- | --- | --- | --- | --- |
| WordPress | awesomemotive/easy-digital-downloads 3.7.3 | WordPress | 4550 | 127 | 926 | bool 8, email 5, enum 1, html 1, int 78, key 31, text 161, unknown 640, url 1 | 5 |
| Joomla | joomla-extensions/weblinks 5.2.1 | Joomla | 111 | 13 | 42 | int 10, key 3, text 2, unknown 27 | 0 |
| Drupal | drupal-graphql/graphql 8.x-4.14 | Drupal | 306 | 9 | 2 | unknown 2 | 0 |
| phpBB | phpbb-extensions/boardrules 3.1.0 | phpBB | 289 | 1 | 18 | bool 3, int 5, text 10 | 0 |
| MediaWiki | wikimedia/mediawiki-extensions-PageForms REL1_43 | MediaWiki | 164 | 14 | 127 | bool 42, text 17, unknown 68 | 0 |
| TYPO3 | georgringer/news 14.1.2 | TYPO3 | 216 | 0 | 9 | int 4, unknown 5 | 0 |

Before 0.2.35 the same extensions gave Drupal 0 inputs, phpBB none of its files
read, MediaWiki 0 inputs and TYPO3 0 inputs. A TYPO3 Extbase plugin
answers on the pages an editor puts it on: its inputs are read, its
entry points are the site's pages.

## Results (0.2.14)

Run again for 0.2.15 (23 Nov 2026): the same numbers on every line, all
guarantees hold (0.2.15 adds the panel's assistant and does not change
the gateway or the agent's maps). Run again for 0.2.16 to 0.2.25: the same.
Run again for 0.2.26 to 0.2.30 (24 Nov 2026; 0.2.27 and 0.2.28 add the panel's security scans and their drafts, 0.2.29 the source reader, used by a discovery only when the site has one, 0.2.30 the browser step's time limit and progress: none changes the gateway or the maps the agent writes without them): attacks on path parameters blocked
110 of 160 instead of 103 (WordPress 96 of 140, Drupal 14 of 20: the
templates' slugs now refuse spaces, `;`, `|` and `$( )`), every page still
the same as on the site, every other line the same, all guarantees hold.
Run again for 0.2.31 (24 Nov 2026; the source reader for every CMS, map
exports and the agent's check of drafts change neither the gateway nor
the maps the discovery writes): the same numbers on every line, all
guarantees hold.
Run again for 0.2.32 (25 Nov 2026; the AI security study and the source
reader's closer reading change neither the gateway nor the maps): the
same numbers on every line, all guarantees hold.
Run again for 0.2.33 (25 Nov 2026; the study after the check, the Logs'
served bars and the report's kept-out total change neither the gateway nor
the maps): every attack line the same, every page visited the same as on
the site (174 of 174), all guarantees hold. Run again for 0.2.34 (the
Logs chart only): the same numbers on every line, all guarantees hold.

Five sites: WordPress 6.9, Drupal 11.4, the custom intranet (generic
profile), phpBB 3.3, MediaWiki 1.43. Joomla, OpenCart, PrestaShop, TYPO3
and Magento need a build (npm) or MariaDB the study's machine did not
have: they are studied where the full lab runs. "Before" is 0.2.13 on the
same sites.

| Measure | 0.2.14 | 0.2.13 |
| --- | --- | --- |
| Pages the same as on the site | 161 of 161 | 162 of 162 |
| Pages served by a precise entry | 159 of 161 (99%) | 91 of 162 (56%) |
| Attacks on typed values (ids, numbers, lists, emails) blocked | 540 of 540 | 160 of 160 |
| The same with an extra key (`&zz=1`) blocked | 500 of 500 | 0 of 120 |
| Attacks on path parameters (`/{slug}/`…) blocked | 103 of 160 (7 neutralised) | 32 of 160 (5 neutralised) |
| Attacks on text with a pattern (search, website) blocked | 36 of 60 | 20 of 40 |
| Attacks on free text (comments, messages, titles) blocked | 302 of 680 | 90 of 240 |
| The same with an extra key blocked | 236 of 520 | 0 of 60 |
| Attacks in a key no entry declares | 0 of 540: the area's | 0 of 300 |
| Attacks on paths only the area serves | 0 of 300: the area's | 0 of 300 |
| Attacks on the protocol (13 per site) | 56 blocked, 9 neutralised, 0 through | the same |

More requests are measured in 0.2.14 because more of each site is served
by precise entries: in 0.2.13 phpBB's and MediaWiki's pages (`viewtopic.php`,
`index.php`) had routes that never served them, so their inputs were
unchecked area traffic.

Reading:

- **Typed values are closed**, extra key included. Before 0.2.14 any
  undeclared key sent the request to the area, where values are not
  checked: `/?p=1' OR '1'='1&zz=1` reached WordPress. Now the area still
  serves such a request, so the site keeps working, but the keys the page
  declares are checked first, and so are their aliases (`page.id` for
  `page_id`).
- **Path values with characters no page name has** are refused instead of
  reaching the area. Values without them that fit no entry
  (`; cat /etc/passwd` as a slug: `/` splits it) still go to the area.
- **Free text passes by nature**: a comment must accept quotes, `;` and
  `|`. Equalizer refuses markup and template syntax where the field says
  so; the rest is the application's job.
- **The area is the remaining surface**: keys no entry declares and paths
  no entry knows reach the site rebuilt but with their values unchecked.
  It shrinks as the agent types what the site uses (`query_added`) and
  what visitors use (`visitors_added`, docs/agent.md).

The study also found why so much was area traffic: the agent gave a
page's keys to a route whose types refuse that page (`/notice.php?id=1`
gave `id` to `/{slug}`, which never serves a name with a dot), so the page
kept going to the area. It now gives them only to a route that serves the
page, else makes one.

## Limits

- Small sites; no plugin with a known vulnerability. OpenCart, PrestaShop
  and Magento are not measured where MariaDB is missing; their request
  readers are covered by the reader's tests.
- 20 payloads per point: broad, not exhaustive. No business-logic tests
  (another user's object, permissions) and no attack as a logged-in user.
- Forms a visitor cannot open, or where a plain value does not reach the
  site, are listed as not measured.
- Direct mode on one gateway: the connector does not change what reaches
  the site.
