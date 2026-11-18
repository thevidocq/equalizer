import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import idor  # noqa: E402


def api(id_, path, **kw):
    return {"id": id_, "path": path, "method": kw.pop("method", "GET"), "roles": kw.pop("roles", ["citizen"]), **kw}


class IdorTest(unittest.TestCase):
    def doc(self, detail_authz=None):
        det = api("permit_get", "/api/permits/{pid}", params={"pid": {"type": "int"}},
                  response={"fields": {"id": {"type": "int"}}})
        if detail_authz is not None:
            det["authz"] = detail_authz
        return {"api": [
            api("permits", "/api/permits", response={"list": True, "fields": {"id": {"type": "int"}, "title": {"type": "string"}}}),
            det,
            api("save", "/api/permits/{pid}", method="PUT", params={"pid": {"type": "int"}}, response={"fields": {}}),
            api("tags", "/api/tags", response={"list": True, "fields": {"id": {"type": "string"}}}),
        ]}

    def test_list_and_detail(self):
        d = self.doc()
        got = idor.propose(d)
        self.assertEqual(got, [{"list": "permits", "detail": "permit_get", "field": "id", "roles": ["citizen"]}])
        self.assertEqual(d["api"][0]["response"]["issues"], {"id": "permit_get.params.pid"})
        self.assertEqual(d["api"][1]["authz"], {"citizen": {"check": "owned_by_user", "target": "params.pid"}})
        self.assertNotIn("authz", d["api"][2])  # PUT is not a detail page
        self.assertEqual(idor.propose(d), [])  # nothing new the second time

    def test_human_choice_kept(self):
        d = self.doc({"citizen": {"check": "always"}})
        self.assertEqual(idor.propose(d), [])
        self.assertNotIn("issues", d["api"][0]["response"])
        self.assertEqual(d["api"][1]["authz"], {"citizen": {"check": "always"}})

    def test_types_must_match(self):
        d = self.doc()
        d["api"][1]["params"]["pid"]["type"] = "slug"
        self.assertEqual(idor.propose(d), [])


if __name__ == "__main__":
    unittest.main()
