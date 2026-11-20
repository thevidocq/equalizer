"""Helpers of the end-to-end tests: the panel API as the browser uses it,
authenticator codes, the lab's mailbox and the connector process."""
import base64
import email
import hashlib
import hmac
import os
import pathlib
import re
import struct
import subprocess
import time

import requests

LAB = pathlib.Path(__file__).resolve().parent.parent
RUN = LAB / ".run"
CA = str(RUN / "ca.pem")
PANEL = "https://panel.lab.test"
SITE = "https://www.acme.test"
# The other legacy sites: public domain, app id, template, address on the
# legacy server, a word only their home page has.
SITES = {
    "joomla": ("news.acme.test", "news", "127.0.0.1:18091", "Acme News"),
    "drupal": ("docs.acme.test", "docs", "127.0.0.1:18092", "Acme Docs"),
    "generic": ("intranet.acme.test", "intranet", "127.0.0.1:18093", "Acme Intranet"),
    # The web applications that prove the agent's other profiles (webapps.sh).
    "opencart": ("cart.acme.test", "cart", "127.0.0.1:18101", "Acme Cart"),
    "prestashop": ("shop.acme.test", "shop", "127.0.0.1:18102", "Acme Shop"),
    "phpbb": ("forum.acme.test", "forum", "127.0.0.1:18103", "Acme Forum"),
    "mediawiki": ("wiki.acme.test", "wiki", "127.0.0.1:18104", "Acme Wiki"),
    "typo3": ("site.acme.test", "site", "127.0.0.1:18105", "Acme Site"),
    "magento": ("store.acme.test", "store", "127.0.0.1:18106", "Acme Store"),
}
# What the connector may reach: WordPress and the other sites.
ALLOW = ",".join(["127.0.0.1:18090"] + [s[2] for s in SITES.values()])
# The intranet is served in http, as it was before Equalizer.
HTTP_SITES = {"intranet.acme.test"}


def site_certificate(*names: str, days: int = 90) -> str:
    """The legacy site's own certificate, as its customer uploads it: issued
    here by the lab CA (a company CA), with the CA and the private key, PEM."""
    import datetime
    from cryptography import x509
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import ec
    from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID
    ca_pem = (RUN / "ca.pem").read_bytes()
    ca = x509.load_pem_x509_certificate(ca_pem)
    ca_key = serialization.load_pem_private_key((RUN / "ca-key.pem").read_bytes(), None)
    key = ec.generate_private_key(ec.SECP256R1())
    now = datetime.datetime.now(datetime.timezone.utc)
    cert = (x509.CertificateBuilder().subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, names[0])]))
            .issuer_name(ca.subject).public_key(key.public_key()).serial_number(x509.random_serial_number())
            .not_valid_before(now - datetime.timedelta(hours=1)).not_valid_after(now + datetime.timedelta(days=days))
            .add_extension(x509.SubjectAlternativeName([x509.DNSName(n) for n in names]), critical=False)
            .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]), critical=False)
            .add_extension(x509.AuthorityKeyIdentifier.from_issuer_subject_key_identifier(
                ca.extensions.get_extension_for_class(x509.SubjectKeyIdentifier).value), critical=False)
            .add_extension(x509.SubjectKeyIdentifier.from_public_key(key.public_key()), critical=False)
            .sign(ca_key, hashes.SHA256()))
    return (cert.public_bytes(serialization.Encoding.PEM) + ca_pem + key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption())).decode()
os.environ["NO_PROXY"] = "127.0.0.1,localhost,.lab.test,.acme.test,acme.test"
os.environ["no_proxy"] = os.environ["NO_PROXY"]
# requests prefers these over a session's own CA: the lab CA must win.
for _v in ("REQUESTS_CA_BUNDLE", "CURL_CA_BUNDLE"):
    os.environ.pop(_v, None)


def totp(secret: str, at: float | None = None) -> str:
    key = base64.b32decode(secret.upper() + "=" * (-len(secret) % 8))
    counter = int((at or time.time()) // 30)
    digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
    o = digest[-1] & 0x0F
    return f"{(struct.unpack('>I', digest[o:o + 4])[0] & 0x7FFFFFFF) % 1_000_000:06d}"


class Panel:
    """A person using the panel: one session, the CSRF header on writes."""

    def __init__(self):
        self.s = requests.Session()
        self.s.verify = CA
        self.s.headers["X-Eq-Panel"] = "1"
        self.secret = ""

    def call(self, method: str, path: str, body=None, want: int | None = None):
        r = self.s.request(method, PANEL + "/v1" + path, json=body, timeout=30)
        if want is not None and r.status_code != want:
            raise AssertionError(f"{method} {path}: {r.status_code} {r.text[:300]}")
        return r.json() if r.content and r.headers.get("Content-Type", "").startswith("application/json") else None

    def login(self, email_: str, password: str, secret: str = ""):
        self.call("POST", "/auth/login", {"email": email_, "password": password}, 200)
        if secret:
            self.secret = secret
            self.call("POST", "/auth/mfa/verify", {"code": totp(secret)}, 200)

    def enrol(self):
        """First login: link the authenticator app."""
        setup = self.call("POST", "/auth/mfa/setup", None, 200)
        self.secret = setup["secret"]
        self.call("POST", "/auth/mfa/enable", {"code": totp(self.secret)}, 200)
        return self.secret


def mail_link(to: str, marker: str, since: float, timeout: float = 20) -> str:
    """The link containing marker in the newest email to `to` after since."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        for f in sorted((RUN / "mail").glob("*.eml"), reverse=True):
            if int(f.stem) / 1e9 < since:
                break
            msg = email.message_from_bytes(f.read_bytes())
            if to not in msg.get("X-Lab-To", ""):
                continue
            body = msg.get_payload(decode=True).decode(errors="replace") if not msg.is_multipart() else \
                msg.get_payload(0).get_payload(decode=True).decode(errors="replace")
            m = re.search(r"https://\S*" + re.escape(marker) + r"\S*", body)
            if m:
                return m.group(0)
        time.sleep(0.3)
    raise AssertionError(f"no email to {to} with {marker}")


def mails(to: str, since: float) -> list[email.message.Message]:
    out = []
    for f in sorted((RUN / "mail").glob("*.eml")):
        if int(f.stem) / 1e9 >= since:
            msg = email.message_from_bytes(f.read_bytes())
            if to in msg.get("X-Lab-To", ""):
                out.append(msg)
    return out


class Connector:
    """The connector process on the legacy server."""

    def __init__(self, cid: str, token: str, allow: str = ALLOW):
        self.env = dict(os.environ, EQ_RELAY_URL="https://relay.lab.test", EQ_CONNECTOR_ID=cid,
                        EQ_CONNECTOR_TOKEN=token, EQ_ALLOW=allow, EQ_RELAY_CA_FILE=CA)
        self.p = None

    def start(self):
        log = open(RUN / "logs" / "connector.log", "a")
        self.p = subprocess.Popen([str(RUN / "bin" / "connector")], env=self.env, stdout=log, stderr=log)

    def stop(self):
        if self.p:
            self.p.terminate()
            self.p.wait(10)
            self.p = None
