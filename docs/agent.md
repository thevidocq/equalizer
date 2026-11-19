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
| Administration | Never opened | Opened read-only with a test administrator |
| Accounts | One optional test account | Up to three (for example visitor, customer, administrator): who uses what becomes roles in the map |
| Forms | Field names; hidden fields kept on the gateway | Also `type`, `maxlength`, `min`/`max`, `pattern`, `required`, the options of every `<select>` and radio group: tight types without guessing |
| APIs the site declares | Not read | Read: `/wp-json/` (WordPress lists its routes with argument types), Drupal JSON:API, OpenAPI files |
| Map | Precise entries for public pages, everything else in the `/` area | A precise entry for every function understood; the `/` area keeps only what is listed under "still open" |
| Checks | None: a broken page is found by using the site | Every page compared with and without Equalizer; every field tried with the attack set |
| Result | A draft | A draft with a score and the list of what is still open |
| AI | Reviews the structure (masked samples): rename, exclude, narrow | Unchanged (see [AI](#ai)) |

## How a study runs

### 1. Explore

- **Seeds**: `/`, `robots.txt`, `sitemap.xml`, the template's pages, the
  paths real visitors asked for (reported by the gateway, as today), the
  APIs the site declares.
- **Browser**: headless Chrome inside the agent. Every request the browser
  makes goes through a small proxy inside the agent, and from there through
  the connector exactly like today's requests. That proxy is the single
  place where the read-only rules below are enforced: the browser cannot
  bypass it.
- **Passes**: one anonymous, then one per test account. Each pass records
  the pages, the requests the scripts make (with their parameters), the
  forms and their fields, and the links.
- **Limits**: `max_pages`, at most 20 URLs per shape (calendars, searches),
  `EQ_RATE` requests per second, as today.

### 2. Understand

Deterministic, no AI needed:

- **Routes**: varying path segments become typed parameters (`{id}` integer,
  `{slug}`), as today.
- **Values**: every parameter gets the narrowest type that fits all the
  values seen and the form's own attributes: an integer with its range, a
  list of values (`enum`) when the site offers a closed set (a `<select>`,
  links that only ever use `asc` and `desc`), a date, an email, a slug, a
  text with its maximum length.
- **Text fields**: a field whose values never contain markup or SQL
  delimiters gets the gateway's `no_html`, `no_sql_meta` and
  `no_template_meta` rules. A field that must accept them (a rich-text
  editor, a code snippet) does not, and is listed under "still open" with
  the reason.
- **Roles**: a page seen only by the administrator account becomes an entry
  for that role only; a page seen by everyone stays open to everyone.
- **Ownership**: when two accounts open the same path with different ids and
  each sees only their own (`/account/orders/{id}/`), the entry gets the
  map's `owned_by_user` rule: a session opens only the ids the site itself
  showed it.

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

A person reviews it, may change it in the editor, presses **Verify** to run
steps 4 and 5 again on their version, and approves. Nothing is ever applied
without approval.

## Read-only, guaranteed

The agent never changes the legacy site. Three layers:

1. **No writes leave the agent.** The proxy in front of the browser refuses
   every method other than `GET` and `HEAD`, except the login of the test
   accounts (one `POST` to the CMS's login page). Forms are read, never
   submitted.
2. **No action links.** Some links change something with a plain `GET`
   (WordPress's "Trash", "Deactivate", "Log out"). They are never opened: a
   fixed list per CMS plus generic patterns (`delete`, `remove`, `trash`,
   `deactivate`, `logout`, `action=…`, links carrying a one-time token such
   as `_wpnonce`). They are listed in the report.
3. **Checks stop at the gateway.** Preview only serves `GET` and `HEAD`;
   everything else is dry-run and never forwarded.

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
    (`Verification`): the **Verify** button;
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
| 3 | Agent: browser and read-only proxy, several accounts, forms and declared APIs, roles and ownership, tighter map | Planned |
| 4 | Agent: the two checks and the score; panel-api verification queue | Planned |
| 5 | Panel: score and "still open" on the Discovery tab, Verify button on drafts | Planned |
| 6 | Lab: studies of WordPress, Joomla, Drupal and the intranet with the expected scores | Planned |

## Mockups

How the panel will show a study (fictitious company Acme; not built yet).

The Discovery tab after a study: the score, what the site does with the rule
written for each function, and what is still open with its reason.

![Discovery tab after a study](screenshots/mockups/study-discovery.png)

A draft changed by hand and verified again: the change broke one function,
and the panel says which and why.

![Verify after a manual change](screenshots/mockups/study-verify.png)
