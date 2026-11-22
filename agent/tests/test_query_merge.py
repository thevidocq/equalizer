"""Query keys a route lacked are added to it (docs/agent.md): the gateway
drops undeclared keys, so a sort or a filter the study never saw would
change the page in silence."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import crawler, fingerprint, mapbuild  # noqa: E402


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
        sort = next(x for x in rep["query_dropped"] if x["key"] == "sort")
        self.assertEqual((sort["route"], sort["clients"], sort["verdict"]), ("/items/{id}", 2, "suspect"))
        self.assertEqual(sort["add"], {"route_id": "item", "key": "sort"})  # one click for the person
        self.assertNotIn("orderby", [x["key"] for x in rep["query_dropped"]])  # added from the site's own links
        self.assertFalse([v for v in rep["visitors_missing"] if v["path"].startswith("/items/{id}")])
        # on a literal route too, and never as a missing page
        clash = next(x for x in rep["query_dropped"] if x["key"] == "id")
        self.assertIsNone(clash["add"])  # never over a path parameter
        self.assertNotIn("page", [x["key"] for x in rep["query_dropped"]])  # declared already
        view = next(x for x in rep["query_dropped"] if x["key"] == "view")
        self.assertEqual(view["add"], {"route_id": "shop", "key": "view"})  # the GET route, not the POST one
        self.assertEqual((view["route"], view["clients"]), ("/shop/", 3))
        self.assertFalse([v for v in rep["visitors_missing"] if v["path"] == "/shop/?view="])
        send = next(r for r in doc["routes"] if r["id"] == "send")
        self.assertNotIn("query", send)  # a route posting to the site is not the agent's to change
        self.assertNotIn("empty", shop["legacy_request"]["query"])  # seen without a value: no evidence


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

    def test_files_refused_are_listed(self):
        from equalizer_agent import crawler, fingerprint, mapbuild
        crawl = crawler.CrawlResult()
        crawl.pages["/"] = crawler.Seen(200, "text/html")
        crawl.assets["/uploads/price-list.csv"] = "text/csv"  # a page of the site links to it
        cur = current()
        cur["assets"] = [{"prefix": "/uploads/", "types": ["image/png"]}]
        job = {"base_url": "http://x.test", "reach": {"mode": "direct", "address": "127.0.0.1:1"},
               "unmapped": [{"method": "GET", "path": "/uploads/orders.csv", "clients": 1},
                            {"method": "GET", "path": "/uploads/evil.js", "clients": 4},
                            {"method": "GET", "path": "/uploads/price-list.csv", "clients": 1}], "current_map": cur}
        doc, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        f = rep["files_refused"]
        # the site's own first, then the suspect ones
        self.assertEqual([(x["path"], x["verdict"]) for x in f], [("/uploads/price-list.csv", "site"),
                         ("/uploads/evil.js", "suspect"), ("/uploads/orders.csv", "suspect")])
        orders = next(x for x in f if x["path"] == "/uploads/orders.csv")
        self.assertEqual(orders["add"], {"prefix": "/uploads/", "type": "text/csv"})
        self.assertIn("every text/csv file under /uploads/", orders["why"])  # what a click would open
        self.assertIsNone(next(x for x in f if x["path"] == "/uploads/evil.js")["add"])  # a script: never
        self.assertEqual(next(a for a in doc["assets"] if a["prefix"] == "/uploads/")["types"], ["image/png"])


if __name__ == "__main__":
    unittest.main()
