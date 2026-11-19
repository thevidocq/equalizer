"""A tenant's model server must not lead the agent into the platform's own
networks."""
import http.server
import os
import ssl
import subprocess
import tempfile
import threading
import unittest
from unittest import mock

from equalizer_agent import llm


class Redirecting(http.server.BaseHTTPRequestHandler):
    def do_POST(self):  # noqa: N802
        self.send_response(302)
        self.send_header("Location", "http://169.254.169.254/latest/meta-data/")
        self.end_headers()

    def log_message(self, *args):
        pass


class TenantModelServer(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.dir = tempfile.mkdtemp()
        key, crt = os.path.join(cls.dir, "k.pem"), os.path.join(cls.dir, "c.pem")
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", key, "-out", crt,
                        "-days", "1", "-subj", "/CN=localhost"], check=True, capture_output=True)
        cls.srv = http.server.HTTPServer(("127.0.0.1", 0), Redirecting)
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(crt, key)
        cls.srv.socket = ctx.wrap_socket(cls.srv.socket, server_side=True)
        threading.Thread(target=cls.srv.serve_forever, daemon=True).start()
        cls.url = f"https://localhost:{cls.srv.server_address[1]}/v1"
        original = ssl.create_default_context
        cls.trust = mock.patch.object(llm.ssl, "create_default_context", lambda: original(cafile=crt))

    @classmethod
    def tearDownClass(cls):
        cls.srv.shutdown()

    def test_private_address_refused_after_connecting(self):
        with self.trust, self.assertRaisesRegex(llm.LLMError, "not public"):
            llm.OpenAICompatible("m", "", self.url).complete("s", "u")

    def test_redirect_not_followed(self):
        with self.trust, self.assertRaisesRegex(llm.LLMError, "redirects are not followed"):
            llm.OpenAICompatible("m", "", self.url, public_only=False).complete("s", "u")


if __name__ == "__main__":
    unittest.main()
