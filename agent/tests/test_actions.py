"""Links that change something are never followed; content slugs that only
contain such a word are pages (docs/agent.md, read-only)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent.webapps.base import is_action  # noqa: E402


class ActionsTest(unittest.TestCase):
    def test_actions(self):
        for u in ("/logout", "/user/delete/5", "/cart/remove-item?id=3", "/remove/x", "/delete-account-now/",
                  "/user-logout-now/", "/account-delete-confirm/", "/?action=delete&id=1",
                  "/wp-login.php?action=logout", "/x?_wpnonce=1", "/blog/reset-your-mind-today/",
                  "/page?do=remove-all-items-now"):
            self.assertTrue(is_action(u), u)

    def test_content_slugs(self):
        for u in ("/blog/how-to-remove-stains/", "/news/big-product-update-2024/",
                  "/shop/oak-drop-leaf-table/", "/guides/how-to-export-wine/"):
            self.assertFalse(is_action(u), u)


if __name__ == "__main__":
    unittest.main()
