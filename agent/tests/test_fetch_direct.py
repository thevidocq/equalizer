import http.server
import socket
import threading
import unittest
from unittest import mock

from equalizer_agent.fetch import Fetcher, ReachError


def _resolves_to(*ips):
    def fake(host, port, *a, **k):
        return [(socket.AF_INET, socket.SOCK_STREAM, 6, "", (ip, port or 0)) for ip in ips]
    return fake


class Echo(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = self.headers.get("Host", "").encode()
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


class DirectTest(unittest.TestCase):
    def test_public_address_allowed_private_refused(self):
        reach = {"mode": "direct", "address": "8.8.8.8:443", "allow": ["8.8.8.8:443"]}
        Fetcher("https://www.acme.test", reach, "acme")  # a public address: allowed
        for dest in ("10.0.0.5:443", "127.0.0.1:80"):
            with self.assertRaises(ReachError):
                Fetcher("https://www.acme.test", {"mode": "direct", "address": dest, "allow": [dest]}, "acme")
    def test_name_resolving_to_public_allowed(self):
        name = {"mode": "direct", "address": "origin.example.com:443", "allow": ["origin.example.com:443"]}
        with mock.patch("equalizer_agent.fetch.socket.getaddrinfo", _resolves_to("8.8.8.8", "1.1.1.1")):
            Fetcher("https://origin.example.com", name, "acme")  # all public: allowed

    def test_name_resolving_to_private_refused(self):
        name = {"mode": "direct", "address": "origin.example.com:443", "allow": ["origin.example.com:443"]}
        for bad in ("10.0.0.5", "169.254.169.254", "127.0.0.1"):
            with mock.patch("equalizer_agent.fetch.socket.getaddrinfo", _resolves_to("8.8.8.8", bad)):
                with self.assertRaises(ReachError):
                    Fetcher("https://origin.example.com", name, "acme")

    def test_name_not_resolving_refused(self):
        name = {"mode": "direct", "address": "www.acme.test:443", "allow": ["www.acme.test:443"]}
        with self.assertRaises(ReachError):  # .test never resolves
            Fetcher("https://www.acme.test", name, "acme")

    def test_pinned_to_the_address_with_the_site_name(self):
        srv = http.server.HTTPServer(("127.0.0.1", 0), Echo)
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        try:
            dest = "127.0.0.1:%d" % srv.server_port
            f = Fetcher("http://www.acme.test", {"mode": "direct", "address": dest, "allow": [dest]}, "acme", direct={dest}, rate=0)
            r = f.get("/")
            self.assertEqual(r.status, 200)
            self.assertEqual(r.body, b"www.acme.test")
        finally:
            srv.shutdown()


if __name__ == "__main__":
    unittest.main()


class ExplainTest(unittest.TestCase):
    def test_provider_refusals_in_words(self):
        from equalizer_agent.llm import _explain
        self.assertIn("API key", _explain(401, b""))
        credit = b'{"error":{"type":"invalid_request_error","message":"Your credit balance is too low"}}'
        self.assertIn("no credit", _explain(400, credit))
        self.assertIn("no credit", _explain(429, b'{"error":{"code":"insufficient_quota"}}'))
        self.assertIn("try again later", _explain(429, b"{}"))
        self.assertIn("does not exist", _explain(404, b""))
        self.assertIn("not answering", _explain(503, b""))
        self.assertNotIn("credit balance is too low", _explain(400, credit))  # never the provider's own text
