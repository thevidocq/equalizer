"""Query keys a route lacked are added to it (docs/agent.md): the gateway
drops undeclared keys, so a sort or a filter the study never saw would
change the page in silence."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import crawler, fingerprint, mapbuild, slugs  # noqa: E402


def current():
    return {"meta": {"version": 3}, "legacy": {}, "roles": {"anonymous": "guest"},
            "routes": [{"id": "shop_send", "path": "/shop/", "method": "POST", "roles": ["guest"],  # same path, first
                        "legacy_request": {"method": "POST", "path": "/shop/"}},
                       {"id": "shop", "path": "/shop/", "method": "GET", "roles": ["guest"],
                        "legacy_request": {"method": "GET", "path": "/shop/", "query": {"page": "{{query.page}}"}},
                        "query": {"page": {"type": "int", "min": 0, "max": 999, "required": False}}},
                       {"id": "item", "path": "/items/{id}", "method": "GET", "roles": ["guest"],
                        "legacy_request": {"method": "GET", "path": "/items/{{params.id}}"},
                        "params": {"id": {"type": "int", "min": 0, "max": 999999, "required": True}}},
                       {"id": "send", "path": "/send", "method": "GET", "roles": ["guest"],
                        "legacy_request": {"method": "POST", "path": "/send"}}]}


class QueryMergeTest(unittest.TestCase):
    def test_keys_added_to_the_route_there(self):
        crawl = crawler.CrawlResult()
        for pq in ("/shop/?page=2", "/shop/?orderby=price&page=1", "/shop/?orderby=date", "/shop/?filter[color]=red",
                   "/shop/?empty=",
                   "/send?x=1"):
            crawl.pages[pq] = crawler.Seen(200, "text/html")
        job = {"base_url": "http://x.test", "reach": {"mode": "direct", "address": "127.0.0.1:1"},
               "unmapped": [{"method": "GET", "path": "/shop/?orderby=", "clients": 2},
                            # the gateway reports a route's pattern, never a visitor's value
                            {"method": "GET", "path": "/items/{id}?sort=", "clients": 2},
                            {"method": "GET", "path": "/items/{id}?once=", "clients": 1},
                            {"method": "GET", "path": "/shop/?view=", "clients": 3},
                            {"method": "GET", "path": "/items/{id}?id=", "clients": 5},  # a path parameter
                            {"method": "GET", "path": "/shop/?page=", "clients": 5}], "current_map": current()}
        doc, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        shop = next(r for r in doc["routes"] if r["id"] == "shop")
        self.assertEqual(shop["legacy_request"]["query"], {"page": "{{query.page}}", "orderby": "{{query.orderby}}"})
        # typed from the values the site's links use, as for a new route
        self.assertEqual(shop["query"]["orderby"]["type"], "string")
        self.assertIn("no_html", shop["query"]["orderby"]["rules"])
        self.assertLessEqual(shop["query"]["orderby"]["max_length"], 200)
        self.assertEqual(shop["query"]["page"]["max"], 999)  # what was there stays
        self.assertIn({"id": "shop", "keys": ["orderby"]}, rep["query_added"])
        item = next(r for r in doc["routes"] if r["id"] == "item")
        # a key only visitors sent is listed for the person, never added
        self.assertNotIn("query", item)
        self.assertEqual(rep["visitors_added"], [])  # added only once checked on the site (visitors.apply)
        self.assertNotIn("view", shop["legacy_request"]["query"])
        send = next(r for r in doc["routes"] if r["id"] == "send")
        self.assertNotIn("query", send)  # a route posting to the site is not the agent's to change
        self.assertNotIn("empty", shop["legacy_request"]["query"])  # seen without a value: no evidence


class FitTest(unittest.TestCase):
    def test_a_route_whose_types_refuse_the_page_does_not_serve_it(self):
        # the generic template's /{slug} refuses dots: /notice.php?id=1 is not
        # its page, so its key goes to a route of its own, which the gateway
        # really serves (efficacy study, 0.2.14), not to /{slug}
        doc = {"meta": {"version": 1}, "legacy": {}, "roles": {"anonymous": "guest"},
               "types": {"slug": {"base": "string", "pattern": "^[^/.<>{}\"'\\\\]{1,200}$"}},
               "routes": [{"id": "page", "path": "/{slug}", "method": "GET", "roles": ["guest"],
                           "legacy_request": {"method": "GET", "path": "/{{params.slug}}"},
                           "params": {"slug": {"type": "slug"}}}]}
        self.assertIsNone(mapbuild.gateway_route(doc, "/notice.php"))
        self.assertEqual(mapbuild.gateway_route(doc, "/about")["id"], "page")
        crawl = crawler.CrawlResult()
        for pq in ("/notice.php?id=1", "/notice.php?id=2", "/about"):
            crawl.pages[pq] = crawler.Seen(200, "text/html", seen_by={"anonymous"})
        job = {"id": 1, "domain": "intranet.example", "current_map": doc, "unmapped": []}
        d, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        page = next(r for r in d["routes"] if r["id"] == "page")
        self.assertNotIn("id", page.get("query") or {})
        notice = next(r for r in d["routes"] if r["path"] == "/notice.php")
        self.assertEqual(notice["query"]["id"]["type"], "int")


class SeedsTest(unittest.TestCase):
    """What visitors asked for is a page to look at, never evidence."""

    def test_seeds(self):
        from equalizer_agent import discovery
        job = {"unmapped": [
            {"method": "GET", "path": "/about-us/", "clients": 2},
            {"method": "GET", "path": "/one-visitor/", "clients": 1},
            {"method": "GET", "path": "/uploads/orders.csv", "clients": 9},  # a file: listed, not opened
            {"method": "GET", "path": "/shop/?sort=", "clients": 9},  # a key: listed, not opened
            {"method": "GET", "path": "/download/8f3a9c2e7b1d4e6fa1/invoice", "clients": 9},  # a capability URL
            {"method": "GET", "path": "/o/123e4567-e89b-12d3-a456-426614174000", "clients": 9},
            {"method": "GET", "path": "/dl/aB3dE5gH7jK9mN1p_q", "clients": 9},
            {"method": "GET", "path": "/invite/Zk8_Qm2Lx9Pw4Rt7Vn3B", "clients": 9},
            {"method": "GET", "path": "/category/electronics2024", "clients": 2},
            {"method": "GET", "path": "/products/iphone_15_pro_max_256gb/", "clients": 2},  # a title
            {"method": "POST", "path": "/contact/", "clients": 9}]}
        got = discovery.seeds(job, None)
        self.assertIn("/about-us/", got)
        self.assertIn("/products/iphone_15_pro_max_256gb/", got)
        self.assertIn("/category/electronics2024", got)
        for p in ("/one-visitor/", "/uploads/orders.csv", "/shop/?sort=", "/download/8f3a9c2e7b1d4e6fa1/invoice",
                  "/o/123e4567-e89b-12d3-a456-426614174000", "/contact/", "/dl/aB3dE5gH7jK9mN1p_q",
                  "/invite/Zk8_Qm2Lx9Pw4Rt7Vn3B"):
            self.assertNotIn(p, got)


if __name__ == "__main__":
    unittest.main()


class UnicodeSlugTest(unittest.TestCase):
    def test_the_agents_slug_takes_any_alphabet_and_no_quotes(self):
        doc = {"meta": {"version": 1}, "legacy": {}, "roles": {"anonymous": "guest"},
               "types": {"slug": {"base": "string", "pattern": slugs.PATTERN, "rules": slugs.RULES}},
               "routes": [{"id": "tag", "path": "/tag/{slug}/", "method": "GET", "roles": ["guest"],
                           "legacy_request": {"method": "GET", "path": "/tag/{{params.slug}}/"},
                           "params": {"slug": {"type": "slug"}}}]}
        for ok in ("/tag/%ce%b5%cf%80%ce%af%cf%80%ce%b5%ce%b4%ce%bf/", "/tag/a%C3%A7%C3%A3o/", "/tag/hello-world/"):
            self.assertEqual((mapbuild.gateway_route(doc, ok) or {}).get("id"), "tag", ok)
        for bad in ("/tag/1'OR'1/", "/tag/a%20b/", "/tag/%3Cx%3E/"):
            self.assertIsNone(mapbuild.gateway_route(doc, bad), bad)
