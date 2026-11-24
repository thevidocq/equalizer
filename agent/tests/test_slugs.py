"""The slug and the agent's reading of RE2 patterns (equalizer_agent/slugs.py)."""
import os
import re
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import cluster, slugs  # noqa: E402

ROOT = os.path.join(os.path.dirname(__file__), "..", "..")


class SlugTest(unittest.TestCase):
    def test_any_alphabet(self):
        for ok in ("επίπεδο-ένα", "ação", "中文", "தமிழ்", "hello-world_2", "2024", "コーヒー", "人々"):
            self.assertTrue(slugs.is_slug(ok), ok)

    def test_never_quotes_spaces_dots_or_lookalikes(self):
        for bad in ("1'OR'1", "a b", "x.php", "a/b", "-x", "_x", "abc\n", "ａｄｍｉｎ", "abcʼd", "⒈x", "a\u034fb", "",
                    "𝐚𝐝𝐦𝐢𝐧", "ﬁle", "ſetup", "\u115fadmin", "ℌx", "aⁱb"):
            self.assertFalse(slugs.is_slug(bad), bad)

    def test_the_gateway_refuses_the_same_lookalikes(self):
        # gateway/internal/validate/rules.go holds the same ranges
        src = open(os.path.join(ROOT, "gateway", "internal", "validate", "rules.go"), encoding="utf-8").read()
        block = src[src.index("var lookalikes"):]
        block = block[:block.index("\n}")]
        go = tuple((int(a, 16), int(b, 16)) for a, b in re.findall(r"\{0x([0-9A-Fa-f]+), 0x([0-9A-Fa-f]+)\}", block))
        self.assertEqual(go, slugs.LOOKALIKES)

    def test_cluster_groups_encoded_slugs(self):
        # one Greek tag must not keep the others from becoming /tag/{slug}/
        out = cluster.patterns(["/tag/a/", "/tag/b/", "/tag/%ce%b5%cf%80/"])
        self.assertEqual(sorted(out), ["/tag/{slug}/"])


class RE2Test(unittest.TestCase):
    def test_categories_in_and_out_of_classes(self):
        self.assertTrue(slugs.re2_search(r"^[\p{L}]+$", "Ωmega"))
        self.assertFalse(slugs.re2_search(r"^[\p{L}]+$", "123"))   # a digit is not a letter
        self.assertTrue(slugs.re2_search(r"^[^\p{L}]+$", "123"))   # negated class
        self.assertTrue(slugs.re2_search(r"^\p{Nd}+$", "٣٤"))      # Arabic-Indic digits
        self.assertTrue(slugs.re2_search(r"^\P{L}+$", "12-3"))
        self.assertTrue(slugs.re2_search(r"^[\]\p{L}]+$", "]a"))   # escaped ]

    def test_classes_are_ascii_as_in_re2(self):
        self.assertFalse(slugs.re2_search(r"^\d+$", "٣"))
        self.assertFalse(slugs.re2_search(r"^\w+$", "é"))
        self.assertTrue(slugs.re2_search(r"^[^\s]+$", "a\u00a0b"))  # NBSP is not \s in RE2

    def test_dollar_is_the_end_of_the_text(self):
        self.assertFalse(slugs.re2_search(r"^abc$", "abc\n"))

    def test_unknown_syntax_is_left_to_the_gateway(self):
        self.assertIsNone(slugs.re2_search(r"^\p{Greek}+$", "αβ"))
        self.assertIsNone(slugs.re2_search(r"^\pL+$", "a"))
        self.assertIsNone(slugs.re2_search(r"^[[:alpha:]]+$", "a"))

    def test_the_templates_old_negated_slug_still_reads(self):
        rx = "^[^/.<>{}\"'\\\\\\x00-\\x1f\\x7f]{1,200}$"
        self.assertTrue(slugs.re2_search(rx, "chi-siamo"))
        self.assertFalse(slugs.re2_search(rx, "wp-login.php"))


if __name__ == "__main__":
    unittest.main()
