"""A small WordPress-like site for the tests."""

import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HOME = """<html><head><meta name="generator" content="WordPress 5.8"><title>City news</title>
<link rel="stylesheet" href="/wp-content/themes/twenty/style.css?ver=1.2">
<script src="/wp-content/plugins/contact-form-7/js/scripts.js?ver=5.4"></script></head><body>
<a href="/about/">About</a> <a href="/contact/">Contact</a> <a href="/team/">Team</a> <a href="/app/">App</a>
<a href="/category/a/">A</a> <a href="/category/b/">B</a> <a href="/category/c/">C</a>
<a href="/?p=12">Post 12</a> <a href="/?p=15">Post 15</a> <a href="/?p=19">Post 19</a>
<a href="/wp-admin/">Admin</a> <a href="/backup.zip">old backup</a> <a href="/logout">Log out</a>
<a href="https://elsewhere.example/">External</a>
<img src="/wp-content/uploads/2024/05/a.jpg">
<form action="/" method="get"><input name="s"></form>
<form action="/wp-comments-post.php" method="post"><textarea name="comment" required></textarea>
<input name="author" maxlength="80"><input type="email" name="email"><input type="hidden" name="comment_post_ID" value="12">
<input type="submit" value="Send"></form>
<form action="/upload.php" method="post"><input type="file" name="doc"><input name="title"></form>
<form action="/signup/" method="post"><input name="first_name" maxlength="60" required>
<input name="nickname"><input type="tel" name="phone"><input type="url" name="site">
<input name="zip" pattern="[0-9]{5}"><input name="bad" pattern="(?=a)b">
<input type="number" name="age" min="18" max="120"><input type="number" name="weight" step="0.1">
<input type="radio" name="plan" value="free"><input type="radio" name="plan" value="pro">
<input type="checkbox" name="news" value="yes"><select name="country"><option value="it">IT</option><option value="fr">FR</option></select>
<textarea name="bio"></textarea><input type="submit"></form>
%s</body></html>"""
LOGIN = """<html><body><form action="/wp-login.php" method="post"><input name="log"><input type="password" name="pwd">
<input type="hidden" name="testcookie" value="1"></form></body></html>"""


class Site:
    def __init__(self):
        self.requests = []
        site = self

        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def _send(self, status, body=b"", ctype="text/html", headers=None):
                self.send_response(status)
                self.send_header("Content-Type", ctype)
                for k, v in (headers or {}).items():
                    self.send_header(k, v)
                self.end_headers()
                self.wfile.write(body if isinstance(body, bytes) else body.encode())

            def do_GET(self):
                site.requests.append(("GET", self.path, self.headers.get("Host")))
                cookie = self.headers.get("Cookie") or ""
                logged = "session=ok" in cookie
                editor = "session=ed" in cookie
                p = self.path.split("?")[0]
                if p == "/":
                    # like most PHP sites: an anonymous session cookie on the first visit
                    extra = ('<a href="/members/area/profile">Members</a>' if logged else "") + \
                        ('<a href="/editors/desk">Desk</a>' if editor else "")
                    return self._send(200, HOME % extra,
                                      headers={"Set-Cookie": "PHPSESSID=anon; Path=/"})
                if p in ("/about/", "/contact/", "/team/", "/category/a/", "/category/b/", "/category/c/", "/events/list"):
                    return self._send(200, "<html><title>%s</title><body>page</body></html>" % p)
                if p == "/app/":
                    return self._send(200, """<html><title>App</title><body><div id=out></div><script>
fetch(['', 'data', 'items'].join('/') + '?page=' + (0 + 1)).then(r => r.json()).then(j => { out.textContent = j.items.length; });
fetch('/api/track', {method: 'POST', body: 'x'});
fetch('/logout');
fetch('https://analytics.example/collect');
</script></body></html>""")
                if p == "/data/items":
                    return self._send(200, '{"items": [1, 2]}', ctype="application/json")
                if p == "/members/area/profile" and logged:
                    return self._send(200, "<html><title>Members</title></html>")
                if p == "/editors/desk" and editor:
                    return self._send(200, "<html><title>Desk</title></html>")
                if p.endswith(".js"):
                    return self._send(200, 'fetch("/wp-json/wp/v2/posts"); var u = "/events/list";', "application/javascript")
                if p == "/wp-login.php":
                    return self._send(200, LOGIN)
                if p == "/robots.txt":
                    return self._send(200, "User-agent: *\nDisallow: /wp-admin/\n", "text/plain")
                return self._send(404, "<html>not found</html>")

            def do_POST(self):
                n = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(n).decode()
                site.requests.append(("POST", self.path, body))
                if self.path == "/wp-login.php" and "log=tester" in body and "pwd=pw&" in body + "&" and "testcookie=1" in body:
                    return self._send(302, "", headers={"Location": "/", "Set-Cookie": "session=ok; Path=/"})
                if self.path == "/wp-login.php" and "log=editor" in body and "pwd=pw2" in body and "testcookie=1" in body:
                    return self._send(302, "", headers={"Location": "/", "Set-Cookie": "session=ed; Path=/"})
                return self._send(200, LOGIN)

        self.server = ThreadingHTTPServer(("127.0.0.1", int(__import__("os").environ.get("FAKE_PORT", "0"))), H)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
