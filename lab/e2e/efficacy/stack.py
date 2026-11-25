"""Brings one site up for the study: the legacy site behind record.php, the
real discovery agent making its map (as the panel would ask it), the map
signed, and the gateway serving it in direct mode."""
import json
import os
import socket
import subprocess
import sys
import time

from sites import OUT, PASSWORD, ROOT

sys.path.insert(0, str(ROOT / "agent"))
from equalizer_agent import config, discovery  # noqa: E402

LAB_DIR = ROOT / "lab" / "e2e" / "efficacy"


def wait(port: int, timeout: float = 30) -> None:
    end = time.time() + timeout
    while time.time() < end:
        try:
            socket.create_connection(("127.0.0.1", port), 1).close()
            return
        except OSError:
            time.sleep(0.2)
    raise RuntimeError(f"nothing listens on {port}")


def legacy(name: str, s: dict) -> subprocess.Popen:
    log = OUT / name / "legacy.jsonl"
    log.write_text("")
    env = {**os.environ, "PHP_CLI_SERVER_WORKERS": "4", "STUDY_LOG": str(log), **s.get("env", {})}
    if s.get("router"):
        env["STUDY_ROUTER"] = str(s["router"])
    p = subprocess.Popen(["php", "-S", f"127.0.0.1:{s['legacy']}", "-t", str(s["root"]), str(LAB_DIR / "record.php")],
                         cwd=str(s.get("cwd", s["root"])), env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    wait(s["legacy"])
    return p


class _Panel:
    def __init__(self, out):
        self.out = out

    def progress(self, *a, **k):
        pass

    def result(self, jid, report=None, document=None, error=None):
        if error:
            raise RuntimeError(error)
        (self.out / "draft.json").write_text(json.dumps(document, indent=1))
        (self.out / "report.json").write_text(json.dumps(report, indent=1))


class _Log:
    def __getattr__(self, n):
        return lambda *a, **k: None


def discover(name: str, s: dict) -> tuple:
    """(draft, report) from the real agent, from the kind's template."""
    out = OUT / name
    kind = name
    tpl = (ROOT / "panel-api/internal/templates/maps" / f"{kind}.json").read_text()
    address = f"127.0.0.1:{s['legacy']}"
    tpl = json.loads(tpl.replace("__BASE_URL__", f"http://{s['host']}").replace("__ADDRESS__", address)
                     .replace("__CONNECTOR__", ""))
    tpl.pop("_template", None)
    tpl["legacy"]["reach"] = {"mode": "direct", "address": address, "allow": [address]}
    tpl["meta"].update({"tenant": "lab", "app": name, "public_domain": s["host"], "public_scheme": "http"})
    job = {"id": 1, "app": name, "tenant": "lab", "domain": s["host"], "kind": kind, "base_url": f"http://{s['host']}",
           "reach": tpl["legacy"]["reach"], "max_pages": 300, "template": tpl, "unmapped": [],
           "accounts": [{"label": "test", "username": s["account"], "password": PASSWORD}] if s["account"] else []}
    summary = source(name, s)
    if summary is not None:
        job.update(source=summary, source_status="used")
    cfg = config.Config(direct={address}, rate=50, browser="off")
    discovery.run(job, cfg, _Panel(out), _Log())
    return json.loads((out / "draft.json").read_text()), json.loads((out / "report.json").read_text())


def gateway(name: str, s: dict, draft: dict) -> subprocess.Popen:
    out = OUT / name
    tool, gw = OUT / "eqtool", OUT / "gateway"
    if not (OUT / "map.key").exists():
        subprocess.run([str(tool), "keygen", "-out", str(OUT / "map.key")], check=True, capture_output=True)
        subprocess.run([str(tool), "formkey", "-out", str(OUT / "form.key")], check=True, capture_output=True)
    doc = dict(draft)
    doc["meta"] = {**doc["meta"], "status": "published", "version": 1}
    (out / "map.json").write_text(json.dumps(doc))
    subprocess.run([str(tool), "sign", "-key", str(OUT / "map.key"), "-map", str(out / "map.json"), "-key-id", "k1",
                    "-out", str(out / "current.json")], check=True, capture_output=True)
    env = {**os.environ, "EQ_PUBLIC_ADDR": f"127.0.0.1:{s['gateway'] + 1000}", "EQ_PUBLIC_HTTP_ADDR": f"127.0.0.1:{s['gateway']}",
           "EQ_ADMIN_ADDR": f"127.0.0.1:{s['gateway'] + 2000}", "EQ_MAP_FILE": str(out / "current.json"),
           "EQ_MAP_PUBKEY_FILE": str(OUT / "map.pub"), "EQ_MAP_KEY_ID": "k1", "EQ_FORM_KEY_FILE": str(OUT / "form.key"),
           "EQ_TRUSTED_PROXIES": "127.0.0.1/32"}
    log = open(out / "gateway.log", "w")
    p = subprocess.Popen([str(gw)], env=env, stdout=log, stderr=log)
    wait(s["gateway"])
    return p


def build() -> None:
    """The gateway and eqtool of this checkout."""
    OUT.mkdir(parents=True, exist_ok=True)
    subprocess.run(["go", "build", "-o", str(OUT / "gateway"), "./cmd/gateway"], cwd=ROOT / "gateway", check=True)
    subprocess.run(["go", "build", "-o", str(OUT / "eqtool"), "./cmd/eqtool"], cwd=ROOT / "tools", check=True)
    subprocess.run(["go", "build", "-o", str(OUT / "source-summary"), "./cmd/source-summary"], cwd=ROOT / "source-reader",
                   check=True)


def source(name: str, s: dict):
    """The source reader's summary of the site's code, as the panel hands it
    to the discovery (STUDY_SOURCE=off: the surface only)."""
    if os.environ.get("STUDY_SOURCE") == "off":
        return None
    root = s.get("source_root", s["root"])
    r = subprocess.run([str(OUT / "source-summary"), str(root)], capture_output=True, text=True, timeout=600)
    if r.returncode:
        raise RuntimeError(f"{name}: the source reader failed: {r.stderr.strip()[:200]}")
    (OUT / name / "source.json").write_text(r.stdout)
    return json.loads(r.stdout)
