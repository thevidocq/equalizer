"""What a web application profile says, and the rules every site shares.

A profile describes one kind of web application (a CMS, a shop, a wiki, a
forum) to the parts of the agent that do not know any by name: how to
recognise it, where a test account logs in, where its administration is
and whether the agent may open it, which of its pages act when they are
only opened. Most of a profile is data; code only where an application
behaves differently (Joomla's administration has a login of its own)."""

import re
from dataclasses import dataclass


@dataclass(frozen=True)
class Profile:
    name: str  # the site kind in the panel (SiteKind)
    title: str
    generator: re.Pattern | None = None  # <meta name="generator">, version in group 1
    paths: re.Pattern | None = None  # URLs only this application uses
    components: re.Pattern | None = None  # extensions: group 1 kind, group 2 name, group 3 version (optional)
    login: str = ""  # where a test account logs in (path?query)
    admin: tuple = ()  # its administration: never opened anonymously
    admin_home: tuple = ()  # where a test account's pass starts in it
    admin_login: str = ""  # an administration with a login of its own
    # The agent opens the administration read-only only when the lab proved
    # that viewing it changes nothing (lab/e2e/tests/test_06_study.py).
    admin_read_only: bool = False
    never: tuple = ()  # pages that act when they are only opened
    tested: str = ""  # what the lab proves about this profile, in words ("" = not tested yet)
    note: str = ""  # what to know, in words


# Everyone's usual suspects: never opened, whatever the application.
SENSITIVE = (
    r"^/(phpmyadmin|pma|adminer|cpanel|plesk|webmail|server-status|server-info|cgi-bin)(/|$)",
    r"(^|/)\.(git|svn|hg|env|htaccess|htpasswd|ds_store)", r"\.(bak|old|orig|swp|sql|tar|tgz|gz|zip|rar|7z|log)$",
    r"(^|/)(phpinfo|info|test|debug|install|setup|config|configuration|settings)\.php$",
    r"^/(backup|backups|dump|private|tmp|temp|vendor|node_modules)(/|$)",
)

# Links that change something, whatever the application: never followed.
ACTIONS = re.compile(r"(logout|log-out|signout|sign-out|delete|remove|destroy|unsubscribe|reset|drop|purge|"
                     r"action=(del|delete|remove|logout|edit|update))", re.I)
# More of them, as whole words (a slug such as /running-tips/ is not "run").
ACTION_WORDS = re.compile(
    r"(^|[/?&=_.\-])(trash|untrash|spam|unspam|approve|unapprove|activate|deactivate|enable|disable|install|"
    r"uninstall|upgrade|update|publish|unpublish|archive|checkin|checkout|lock|unlock|block|unblock|cancel|empty|"
    r"restore|duplicate|clone|export|import|flush|clear|rebuild|run|execute|toggle|switch|impersonate|"
    r"login_as|revoke|regenerate|resend|confirm|send)($|[/?&=_.\-])", re.I)
# One-time tokens: a link carrying one acts on the site (Joomla also checks
# an item out when its edit form opens).
TOKENS = re.compile(r"[?&](_wpnonce|nonce|_nonce|token|_token|csrf|csrf_token|form_token|form_key|task|hash)=|"
                    r"[?&][0-9a-f]{32}=1|[?&]layout=edit", re.I)


def is_action(path_query: str) -> bool:
    return bool(ACTIONS.search(path_query) or ACTION_WORDS.search(path_query) or TOKENS.search(path_query))
