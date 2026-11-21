import http.server
import threading
import unittest

from equalizer_agent.fetch import Fetcher, ReachError


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
        name = {"mode": "direct", "address": "www.acme.test:443", "allow": ["www.acme.test:443"]}
        with self.assertRaises(ReachError):  # a name could resolve to the platform's network
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
