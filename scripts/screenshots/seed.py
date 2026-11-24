"""Fills an empty panel with the demo organisation the screenshots show:
Acme, its people, connector, two sites, discoveries from real studies of
the lab's WordPress and custom intranet (fixtures/), published maps, a verification, a
certificate, a source reader with a read. Everything goes through the
panel's own API (the internal one for what the agent and the source
reader would send), so the pages show what the product really writes.

    python3 scripts/screenshots/seed.py <panel-api URL> <internal token> <panel-api log file>

The platform administrator must exist already (panel-api create-admin,
root@acme.example). Invitation links are read from the log
(EQ_MAIL_MODE=log). Prints the accounts as JSON for shoot.py."""

import json
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(HERE, "..", "..")
sys.path.insert(0, os.path.join(ROOT, "agent"))

PASSWORD = "demo-password-2026"
ROOT_ADMIN = ("root@acme.example", PASSWORD)


class Client:
    """A panel session: the cookie is kept by hand (it is Secure, and the
    panel runs in http here)."""

    def __init__(self, base, token=None):
        self.base, self.cookie, self.token = base.rstrip("/"), None, token

    def call(self, method, path, body=None, want=(200, 201, 202, 204)):
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(self.base + path, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        req.add_header("X-Eq-Panel", "1")
        if self.cookie:
            req.add_header("Cookie", self.cookie)
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(req) as r:
                status, raw, headers = r.status, r.read(), r.headers
        except urllib.error.HTTPError as e:
            status, raw, headers = e.code, e.read(), e.headers
        for c in headers.get_all("Set-Cookie") or []:
            self.cookie = c.split(";")[0]
        if status not in want:
            sys.exit(f"{method} {path}: {status} {raw[:300]!r}")
        return json.loads(raw) if raw else None


def login(base, email, password):
    c = Client(base)
    c.call("POST", "/v1/auth/login", {"email": email, "password": password})
    return c


def invitation(log, email):
    """The newest invitation link for email in panel-api's log."""
    for _ in range(50):
        with open(log) as f:
            lines = f.readlines()
        for line in reversed(lines):
            if email in line and "/invite/" in line:
                return re.search(r"/invite/([A-Za-z0-9_-]+)", line).group(1)
        time.sleep(0.2)
    sys.exit("no invitation for " + email)


def join(base, log, email):
    c = Client(base)
    c.call("POST", f"/v1/invitations/{invitation(log, email)}/accept", {"password": PASSWORD})
    return c


def certificate(names):
    """A certificate for the site's names, signed by a demo CA, with its key."""
    d = tempfile.mkdtemp()
    run = lambda *a: subprocess.run(a, cwd=d, check=True, capture_output=True)
    run("openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-days", "3650",
        "-subj", "/CN=Acme Demo CA", "-keyout", "ca.key", "-out", "ca.crt")
    run("openssl", "req", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-subj", "/CN=" + names[0],
        "-keyout", "site.key", "-out", "site.csr")
    with open(os.path.join(d, "ext"), "w") as f:
        f.write("subjectAltName=" + ",".join("DNS:" + n for n in names) + "\nextendedKeyUsage=serverAuth\n")
    run("openssl", "x509", "-req", "-in", "site.csr", "-CA", "ca.crt", "-CAkey", "ca.key", "-CAcreateserial",
        "-days", "80", "-extfile", "ext", "-out", "site.crt")
    return "".join(open(os.path.join(d, n)).read() for n in ("site.crt", "ca.crt", "site.key"))


def fixture(name):
    with open(os.path.join(HERE, "fixtures", name)) as f:
        return json.load(f)


def discovery(alex, internal, app, start, study="generic", drop=()):
    """A discovery the agent ends with a real study's report and map (the
    lab's custom intranet by default)."""
    d = alex.call("POST", f"/v1/apps/{app}/discoveries", start)
    job = internal.call("POST", "/v1/internal/discoveries/claim")
    assert job and job["id"] == d["id"], job
    doc = json.loads(json.dumps(job.get("current_map") or job["template"]))
    doc.update(fixture(study + "-map.json"))
    doc["routes"] = [r for r in doc["routes"] if r["id"] not in drop]
    report = fixture(study + "-report.json")
    report["ai"] = {"used": True}
    internal.call("POST", f"/v1/internal/discoveries/{d['id']}/result", {"report": report, "document": doc})
    return d["id"]


SCORE = {"functions_total": 47, "functions_identical": 47, "functions_changing": 2,
         "inputs_total": 31, "inputs_protected": 29, "area_percent": 2.1}


def verify(internal, correct=False, open_=()):
    """The agent's check of the newest draft; with correct, it narrows a
    key to the values the site uses and hands over the corrected version."""
    v = internal.call("POST", "/v1/internal/verifications/claim")
    if not v:
        return
    out = {"score": dict(SCORE), "open": list(open_)}
    if open_:
        out["broken"] = [{"path": "/search.php?q=%27", "entry": "search_php", "difference": "refused",
                          "detail": "a search with an apostrophe is refused: the field refuses SQL characters"}]
    if correct:
        doc = v["document"]
        for r in doc["routes"]:
            if r["id"] == "notice_php":
                r["query"]["id"].update(min=1, max=500)
        out.update(document=doc, before=dict(SCORE, inputs_protected=28), score=dict(SCORE, inputs_protected=29),
                   corrections=[{"entry": "notice_php.id", "change": "narrowed", "detail": "int 0–999999999 → int 1–500",
                                 "reason": "every notice the site links to is between 1 and 500"}],
                   broken=[{"path": "/search.php?q=%27", "entry": "search_php", "difference": "refused",
                            "detail": "a search with an apostrophe is refused: the field refuses SQL characters"}])
    internal.call("POST", f"/v1/internal/verifications/{v['id']}/result", out)


def main(base, token, log):
    internal = Client(base, token)
    root = login(base, *ROOT_ADMIN)
    root.call("POST", "/v1/tenants", {"id": "acme", "name": "Acme", "admin_email": "alex.morgan@acme.example",
                                      "admin_name": "Alex Morgan"})
    alex = join(base, log, "alex.morgan@acme.example")
    alex.call("PATCH", "/v1/auth/me", {"time_zone": "Europe/Rome"}, want=(200, 204))
    alex.call("POST", "/v1/tenants/acme/invitations", {"email": "jamie.lee@acme.example", "name": "Jamie Lee", "role": "viewer"})
    join(base, log, "jamie.lee@acme.example")
    alex.call("POST", "/v1/tenants/acme/invitations", {"email": "chris.park@acme.example", "name": "Chris Park", "role": "admin"})

    alex.call("POST", "/v1/tenants/acme/connectors", {"id": "cn-web01"})
    alex.call("PUT", "/v1/tenants/acme/llm", {"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-ant-demo-key"})
    alex.call("POST", "/v1/tenants/acme/apps", {"id": "blog", "name": "Company blog", "public_domain": "blog.acme.example",
                                                "kind": "wordpress"})
    alex.call("POST", "/v1/tenants/acme/apps", {"id": "portal", "name": "Customer portal", "public_domain": "portal.acme.example",
                                                "kind": "generic"})
    reach = {"connector_id": "cn-web01", "address": "127.0.0.1:8443"}

    # The source reader of the blog: settings, token, hello with its key, a read
    alex.call("PUT", "/v1/apps/blog/source", {"method": "sftp", "host": "web01.acme.lan", "username": "eq-reader",
                                              "path": "/var/www/blog"})
    reader = Client(base, alex.call("POST", "/v1/apps/blog/source/token")["token"])
    reader.call("POST", "/v1/source/hello", {"version": "0.2.29", "public_key":
                "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHk3bXlEZW1vS2V5Rm9yU2NyZWVuc2hvdHNPbmx5 equalizer-source-reader"})

    # First discovery of the blog, with the source: the reader answers first
    blog = dict(reach, source=True, accounts=[{"label": "editor", "username": "editor", "password": "x"}])
    started = alex.call("POST", "/v1/apps/blog/discoveries", blog)
    job = reader.call("POST", "/v1/source/claim")
    with open(os.path.join(ROOT, "schema", "source", "examples", "wordpress.json")) as f:
        reader.call("POST", f"/v1/source/reads/{job['id']}/result", {"summary": json.load(f)})
    claimed = internal.call("POST", "/v1/internal/discoveries/claim")
    assert claimed["id"] == started["id"], claimed
    doc = json.loads(json.dumps(claimed["template"]))
    doc.update(fixture("wordpress-map.json"))
    report = fixture("wordpress-report.json")
    report["ai"] = {"used": True}
    from equalizer_agent import source
    source.apply(report, doc, claimed, {}, {p.split("?")[0] for p in report.get("pages", [])})
    internal.call("POST", f"/v1/internal/discoveries/{started['id']}/result", {"report": report, "document": doc})
    verify(internal)
    alex.call("POST", "/v1/apps/blog/maps/1/publish")

    # The portal: a first map published, then a newer draft waiting for review
    discovery(alex, internal, "portal", reach, drop=("search_php",))
    verify(internal)
    alex.call("POST", "/v1/apps/portal/maps/1/publish")
    discovery(alex, internal, "portal", {})
    verify(internal, open_=fixture("generic-report.json")["open"])

    # A draft of the blog a person wrote, in review, with the AI's summary
    v1 = alex.call("GET", "/v1/apps/blog/maps/1")["document"]
    v1.setdefault("routes", []).append({
        "id": "backup", "path": "/backup/{file}", "method": "GET", "roles": [v1["roles"]["anonymous"]], "render": "faithful",
        "origin": "human", "params": {"file": {"type": "slug", "required": True}},
        "legacy_request": {"method": "GET", "path": "/backup/{{params.file}}"}})
    alex.call("POST", "/v1/apps/blog/maps", {"document": v1, "note": "Add the backup download page"})
    alex.call("POST", "/v1/apps/blog/maps/2/submit")
    review = internal.call("POST", "/v1/internal/reviews/claim")
    if review:
        internal.call("POST", f"/v1/internal/reviews/{review['id']}/result", {
            "summary": "Adds one public route, /backup/{file}, that serves files from the backup folder to anyone.",
            "questions": ["Should backups be public? If not, leave this route out: the site area keeps serving the rest."]})

    alex.call("PUT", "/v1/apps/blog/aliases", {"aliases": ["www.blog.acme.example"]})
    alex.call("PUT", "/v1/apps/blog/certificate", {"pem": certificate(["blog.acme.example", "www.blog.acme.example"])})
    alex.call("PATCH", "/v1/apps/blog", {"legacy_ip": "93.184.215.14"})
    alex.call("PATCH", "/v1/apps/blog", {"discovery_schedule": "weekly", "discovery_hour": 3, "discovery_weekday": 1})
    print(json.dumps({"root": ROOT_ADMIN[0], "admin": "alex.morgan@acme.example", "viewer": "jamie.lee@acme.example",
                      "password": PASSWORD}))


if __name__ == "__main__":
    main(*sys.argv[1:4])
