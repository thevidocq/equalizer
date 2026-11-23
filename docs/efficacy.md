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
  test account, as the panel would ask it (no AI, no browser), and its
  draft is published as it is.
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

## Results (0.2.14)

Run again for 0.2.15 (23 Nov 2026): the same numbers on every line, all
guarantees hold (0.2.15 adds the panel's assistant and does not change
the gateway or the agent's maps). Run again for 0.2.16 and 0.2.17: the same.

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

- Small sites; no plugin with a known vulnerability.
- 20 payloads per point: broad, not exhaustive. No business-logic tests
  (another user's object, permissions) and no attack as a logged-in user.
- Forms a visitor cannot open, or where a plain value does not reach the
  site, are listed as not measured.
- Direct mode on one gateway: the connector does not change what reaches
  the site.
