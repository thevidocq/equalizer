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
            "routes": [{"id": "shop", "path": "/shop/", "method": "GET", "roles": ["guest"],
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
                            {"method": "GET", "path": "/items/{id}?once=", "clients": 1}], "current_map": current()}
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
        self.assertIn({"route": "/items/{id}", "key": "sort", "clients": 2}, rep["query_dropped"])
        self.assertFalse([v for v in rep["visitors_missing"] if v["path"].startswith("/items/{id}")])
        send = next(r for r in doc["routes"] if r["id"] == "send")
        self.assertNotIn("query", send)  # a route posting to the site is not the agent's to change
        self.assertNotIn("empty", shop["legacy_request"]["query"])  # seen without a value: no evidence


if __name__ == "__main__":
    unittest.main()
