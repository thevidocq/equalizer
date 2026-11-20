# Performance: how much Equalizer adds

Measured in the local lab (`lab/e2e/perf.py`) on 2026-11-20, on one
machine with 2 CPUs where everything runs at once: the ten legacy sites
(PHP's own server), the HTTPS front, gateway, relay, connector, Redis and
PostgreSQL. The same pages are opened directly on the legacy server (plain
http) and through Equalizer (https → gateway → relay → connector →
legacy). Maps: the CMS templates published by the onboarding test.

What this measures is Equalizer's own cost. A real deployment adds the
network between the gateway's VPS and the customer's server (a few
milliseconds in the same region, tens across continents), and does not
share CPUs between Equalizer and the legacy site.

## One page (HTML only, connection kept)

47 pages of the ten sites, 30 times each, the two ways alternated; every
answer had the same status both ways.

| Site | Direct (median ms) | Through Equalizer | Added |
| --- | --- | --- | --- |
| WordPress | 53.1 | 59.5 | 3.6 |
| Joomla | 42.7 | 46.5 | 3.8 |
| Drupal | 3.6 | 6.2 | 2.6 |
| Intranet (custom PHP) | 1.4 | 3.5 | 2.1 |
| OpenCart | 42.5 | 47.5 | 5.1 |
| PrestaShop | 55.2 | 62.4 | 6.0 |
| phpBB | 5.5 | 8.3 | 2.9 |
| MediaWiki | 53.5 | 56.2 | 3.5 |
| TYPO3 | 15.9 | 17.7 | 2.1 |
| Magento | 41.5 | 46.0 | 4.2 |

Over all pages: **3.6 ms added (median)**, from −2.6 to 13.7 ms; about 11%
of the page's own time, less on the slower pages. Pages come out slightly
smaller through Equalizer (the legacy hosts, comments and hidden inputs are
removed).

## Whole page in a browser

Chrome, until the load event (images, styles and scripts included), cold
(new profile) and warm (same profile), 5 times each. Through Equalizer
every page and file is served in https (TLS handshakes), directly in plain
http: part of the difference is TLS, which the site would pay anyway when
served in https.

- Cold: typically **+10 to +60 ms** on pages of 40 to 400 ms.
- Warm: typically **+5 to +60 ms**.
- Magento and MediaWiki vary more than the difference itself (the sites
  build their caches while answering).

## Under load

10 clients at once for 20 seconds on a home page, through Equalizer as 250
visitors (each address stays under the templates' 120 pages a minute).

| Site | Direct (pages/s, median ms) | Through Equalizer |
| --- | --- | --- |
| WordPress | 41.2, 249 | 35.2, 287 |
| OpenCart | 49.0, 207 | 40.9, 234 |
| MediaWiki | 37.9, 270 | 31.1, 330 |

About **15–18% fewer pages per second**, no errors. This is pessimistic:
here Equalizer takes CPU from the legacy sites themselves; on a real
deployment it runs on its own server.

## How to run it again

```sh
sudo ./up.sh && python3 -m pytest -q tests/test_01_onboarding.py
python3 perf.py --rounds 30 --browser --load 20 > perf.json
```
