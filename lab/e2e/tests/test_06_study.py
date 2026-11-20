"""The agent studies the four lab sites with a test administrator, through
the connector and with its real browser, verifies the drafts it proposes
on the gateway, and changes nothing on them: the content of every site's
database is the same before and after (docs/agent.md, "Read-only,
guaranteed")."""
import sqlite3
import subprocess
import time

import pytest

import state
from lab import RUN

ACCOUNT = [{"label": "administrator", "username": "editor", "password": "Lab-editor-pass-1"}]

# What a site's content is: tables and columns that only change when
# someone changes something. Counters every visit moves (Joomla's hits,
# last login and visit dates, sessions, caches) are left out on purpose.
WORDPRESS = ["SELECT * FROM wp_posts ORDER BY ID", "SELECT * FROM wp_postmeta ORDER BY meta_id",
             "SELECT * FROM wp_comments ORDER BY comment_ID", "SELECT * FROM wp_terms ORDER BY term_id",
             "SELECT * FROM wp_term_taxonomy ORDER BY term_taxonomy_id",
             "SELECT ID, user_login, user_pass, user_email, user_status FROM wp_users ORDER BY ID",
             "SELECT option_name, option_value FROM wp_options WHERE option_name IN ('active_plugins', 'template', "
             "'stylesheet', 'blogname', 'siteurl', 'home', 'users_can_register', 'default_role', 'permalink_structure') "
             "ORDER BY option_name"]
DRUPAL = ["SELECT * FROM node_field_data ORDER BY nid", "SELECT * FROM node__body ORDER BY entity_id",
          "SELECT uid, name, pass, mail, status FROM users_field_data ORDER BY uid",
          "SELECT * FROM user__roles ORDER BY entity_id", "SELECT * FROM path_alias ORDER BY id",
          "SELECT name, data FROM config ORDER BY name"]
INTRANET = ["SELECT * FROM notices ORDER BY 1", "SELECT * FROM users ORDER BY 1"]
JOOMLA = ["SELECT id, title, introtext, state, catid, checked_out, publish_up FROM jos_content ORDER BY id",
          "SELECT extension_id, enabled FROM jos_extensions ORDER BY extension_id",
          "SELECT id, username, password, block FROM jos_users ORDER BY id",
          "SELECT id, published, checked_out FROM jos_categories ORDER BY id",
          "SELECT id, published, checked_out FROM jos_menu ORDER BY id",
          "SELECT id, published FROM jos_modules ORDER BY id"]


def sqlite_digest(path, queries):
    """{query: rows}: compared whole, so a failure says what changed."""
    db = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
    try:
        return {q: [repr(r) for r in db.execute(q)] for q in queries}
    finally:
        db.close()


def joomla_digest():
    return {q: subprocess.run(["psql", "-h", "127.0.0.1", "-p", "55441", "-U", "postgres", "-d", "joomla", "-At", "-c", q],
                              capture_output=True, text=True, check=True).stdout.splitlines() for q in JOOMLA}


def changes(before, after):
    out = []
    for q in before:
        gone = [r for r in before[q] if r not in after[q]]
        new = [r for r in after[q] if r not in before[q]]
        if gone or new:
            out.append(f"{q[:60]}: -{[g[:200] for g in gone[:3]]} +{[n[:200] for n in new[:3]]}")
    return out


DIGESTS = {
    "blog": lambda: sqlite_digest(RUN / "wp-db" / "wordpress.sqlite", WORDPRESS),
    "news": joomla_digest,
    "docs": lambda: sqlite_digest(RUN / "drupal" / "sites" / "default" / "files" / ".sqlite", DRUPAL),
    "intranet": lambda: sqlite_digest(RUN / "intranet-db" / "intranet.sqlite", INTRANET),
}


def study(app):
    d = state.boss.call("POST", f"/apps/{app}/discoveries", {"use_ai": False, "max_pages": 300, "accounts": ACCOUNT}, 201)
    deadline = time.time() + 900
    while time.time() < deadline:
        got = state.boss.call("GET", f"/apps/{app}/discoveries/{d['id']}", None, 200)
        if got["status"] in ("done", "failed"):
            return got
        time.sleep(3)
    raise AssertionError(f"{app}: the study did not finish")


def verification(app, version):
    """The agent verifies every draft a study proposes (docs/agent.md, steps 4 and 5)."""
    deadline = time.time() + 900
    while time.time() < deadline:
        got = state.boss.call("GET", f"/apps/{app}/maps/{version}/verify", None, 200)
        if got["status"] in ("done", "failed"):
            return got
        time.sleep(3)
    raise AssertionError(f"{app}: the verification did not finish")


@pytest.mark.parametrize("app", ["blog", "news", "docs", "intranet"])
def test_study_changes_nothing(app):
    before = DIGESTS[app]()
    got = study(app)
    assert got["status"] == "done", got.get("error")
    ver = verification(app, got["draft_version"]) if got.get("draft_version") else None
    after = DIGESTS[app]()
    assert not changes(before, after), f"{app}: the study changed the site's content: {changes(before, after)}"
    rep = got["report"]
    assert rep["accounts"] == [{"label": "administrator", "login": "ok"}], rep["accounts"]
    # the real browser opened the pages behind the same guard
    assert rep["browser"]["pages"] > 0, rep["browser"]
    reasons = {o["reason"] for o in rep.get("open", [])}
    assert reasons <= {"not_reached", "action_not_performed", "legacy_error", "cannot_narrow", "accepts_sql", "other"}
    if ver is not None:
        assert ver["status"] == "done", ver.get("error")
        sc = ver["score"]
        assert sc["functions_total"] > 0 and sc["inputs_total"] > 0, sc
        # the test account's pages behind the login are verified too
        assert ver["accounts"] == ["administrator"], ver["accounts"]
        print(app, "verification:", sc, "before corrections:", ver.get("before"), "corrected as version",
              ver.get("corrected_version"), [(c["entry"], c["change"], c.get("detail")) for c in ver.get("corrections", [])][:8],
              "broken:", [(b["path"], b["difference"], b.get("detail", "")[:100]) for b in ver["broken"]][:6])
        if ver.get("corrected_version"):
            fixed = state.boss.call("GET", f"/apps/{app}/maps/{ver['corrected_version']}/verify", None, 200)
            assert fixed["status"] == "done" and fixed["score"]["functions_identical"] >= sc["functions_identical"], fixed
            print(app, "corrected draft:", fixed["score"], [(b["path"], b["difference"], b.get("detail", "")[:100]) for b in fixed["broken"]][:6])
    print(app, "browser:", rep["browser"]["pages"], "pages,", len(rep["browser"]["script_requests"]), "script requests,",
          [k["method"] + " " + k["path"] for k in rep["browser"]["kept_in"]][:10], "kept in")


def latest_report(app):
    got = state.boss.call("GET", f"/apps/{app}/discoveries", None, 200)
    return state.boss.call("GET", f"/apps/{app}/discoveries/{got['items'][0]['id']}", None, 200)["report"]


def test_administration_studied_read_only():
    """Drupal's and Joomla's administrations are reached by the test account
    (Joomla's with its own login) and become precise entries of the draft. WordPress's stays
    closed: it writes to its database when a page is only viewed (the lab
    found it: a Quick Draft, global styles, privacy texts)."""
    for app, prefix in (("docs", "/admin"), ("news", "/administrator/")):
        added = [r["path"] for r in latest_report(app)["routes_added"]]
        assert any(p.startswith(prefix) for p in added), f"{app}: no administration page studied: {added[:20]}"
    added = [r["path"] for r in latest_report("blog")["routes_added"]]
    assert not any(p.startswith("/wp-admin/") for p in added), added
