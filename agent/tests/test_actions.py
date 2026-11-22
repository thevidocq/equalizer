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
                  "/page?do=remove-all-items-now",
                  # titles end with nouns, actions end with verbs: short or verb-ended segments are actions
                  "/cart/item-12-remove", "/wishlist/item-5-remove/", "/subscription/my-plan-cancel",
                  "/user/42/account-disable-now", "/orders/12/order-cancel-confirm", "/posts/7/do-publish-now",
                  "/comments/42/mark-as-spam", "/Account/LogOff", "/users/sign_out", "/x/log_out", "/x/log-off",
                  "/signoff", "/user/signoff", "/SignOff.aspx", "/account/sign-off", "/sign_off", "/doLogoff",
                  "/UserLogOff.aspx", "/Home/LogOffUser", "/x?action=signoff",
                  # all lowercase or uppercase, glued to other words
                  "/dologoff", "/userlogoff.aspx", "/applogoff.php", "/home/logoffuser", "/logoffnow",
                  "/doLOGOFF", "/dosignoff", "/index.php?cmd=dologoff",
                  # a title that ends every session is still an action
                  "/sessions/log-out-all-devices", "/account/sign-out-all-sessions",
                  "/account/sign-out-everywhere-now", "/me/log-off-all-devices",
                  # whatever precedes, whatever the separator or the encoding
                  "/WebLogOff.aspx", "/weblogoff", "/dbLogoff", "/Account/JobLogOff", "/clubLogoff", "/hubLogoff.do",
                  "/codeSignOff", "/ModeSignoff", "/nodesignoff", "/Account/DoLog_Off", "/Home/UserLog-Out",
                  "/userSign_Out", "/doLog-Off", "/AppLog_Out.aspx", "/Account/Log_OffUser", "/x/log_offUser",
                  "/x/log.out", "/sign.out", "/log%2Dout", "/log%5Fout", "/log%252Dout", "?a=log+out",
                  "/log-outnow", "/sign-outall", "/account/sign-off-all-devices", "/my-account-sign-off-now",
                  "/x;jsessionid=1/logout",
                  # other words for it, other languages, lookalikes, file names
                  "/connect/endsession", "/end_session", "/session/end", "/logmeout", "/log-me-out", "/sign_me_out",
                  "/exit", "/quit", "/disconnect", "/session/kill", "/abmelden", "/uitloggen", "/logga-ut",
                  "/deconnexion", "/d%C3%A9connexion", "/cerrar-sesion", "/esci", "/disconnetti",
                  "/%C5%BFignoff", "/%EF%BD%8C%EF%BD%8F%EF%BD%87%EF%BD%8F%EF%BD%95%EF%BD%94",
                  "/Account/LogOffError", "/logoffSetup", "/logout.png", "/logout;.png", "/users/sign_out.txt",
                  # a page about signing off is not studied: safer than a session ended
                  "/sign-off-letter-examples/", "/blog/how-to-sign-off-an-email/"):
            self.assertTrue(is_action(u), u)

    def test_content_slugs(self):
        for u in ("/blog/how-to-remove-stains/", "/news/big-product-update-2024/",
                  "/shop/oak-drop-leaf-table/", "/guides/how-to-export-wine/",
                  # log-off / sign-out spellings are whole words only
                  "/blog-offers/", "/catalog-offers/", "/design-office/", "/blog-offline-mode/",
                  # logoff/signoff never inside a word
                  "/catalogoffers/", "/designoffice/", "/blog/designoffers-2024", "/shop/analogoffset-printing",
                  "/changelogoffset", "/backlogoffice", "/prologoffer", "/vlogoffers", "/resignoffer", "/assignoffice",
                  "/blog-offline-mode/", "/exit-strategy-guide/", "/blog/how-to-quit-smoking/", "/exits/", "/esempio/",
                  "/sessions/list/"):
            self.assertFalse(is_action(u), u)


if __name__ == "__main__":
    unittest.main()
