"""What a web application profile says, and the rules every site shares.

A profile describes one kind of web application (a CMS, a shop, a wiki, a
forum) to the parts of the agent that do not know any by name: how to
recognise it, where a test account logs in, where its administration is
and whether the agent may open it, which of its pages act when they are
only opened. Most of a profile is data; code only where an application
behaves differently (Joomla's administration has a login of its own)."""

import re
import urllib.parse
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
    # HTML files the pages' scripts load as templates: assets, not pages
    # (the gateway serves them as they are, unable to run as a page)
    fragments: tuple = ()
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
# Ending the session (log out, log off, sign out, sign off) is judged on each
# piece of the URL normalised: decoded, lower case, without separators
# (- _ . + spaces), so no spelling escapes (/WebLogOff.aspx, /log%2Dout,
# /Account/DoLog_Off, /log.out). The only exceptions are decided by what
# follows: offer(s), offset, office, offline (catalog-offers, analog-offset,
# design-office, blog-offline). A page that merely talks about signing off is not studied:
# safer than a test account logged out.
_SESSION_END = re.compile(r"logout|signout|(?:log|sign)off(?!ers?|set|ice|line)")
_PIECES = re.compile(r"[/?&=;#]")
_SEPARATORS = re.compile(r"[^a-z0-9]")


def ends_session(path_query: str) -> bool:
    """Whether a URL would end the session, however it is spelled."""
    text = urllib.parse.unquote_plus(urllib.parse.unquote_plus(path_query)).lower()
    return any(_SESSION_END.search(_SEPARATORS.sub("", piece)) for piece in _PIECES.split(text))


ACTIONS = re.compile(r"(logout|signout|"
                     r"delete|remove|destroy|unsubscribe|reset|drop|purge|"
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


_SLUG_WORD = re.compile(r"^[A-Za-z0-9]+$")
# Never part of an innocent slug: a segment holding one is always an action.
STRONG = re.compile(r"(logout|signout|unsubscribe|destroy|delete|purge|impersonate)", re.I)


def _is_action_word(word: str) -> bool:
    return bool(ACTIONS.search(word) or ACTION_WORDS.search(word))


def _content_slug(segment: str) -> bool:
    """A segment that names content: a title of four words or more joined by
    - or _, with at most one action word and never at either end
    ("how-to-remove-stains", "big-product-update-2024"; not "item-12-remove",
    "my-plan-cancel", "account-disable-now", "reset-your-mind-today")."""
    words = re.split(r"[-_]", segment)
    if len(words) < 4 or not all(_SLUG_WORD.match(w) for w in words) or STRONG.search(segment):
        return False
    acts = [i for i, w in enumerate(words) if _is_action_word(w)]
    return len(acts) <= 1 and not any(i in (0, len(words) - 1) for i in acts)


def is_action(path_query: str) -> bool:
    """A link that changes something. The query is always read in full; in
    the path, a segment that is a content slug is not an action whatever
    words it holds."""
    if TOKENS.search(path_query) or ends_session(path_query):
        return True
    path, sep, query = path_query.partition("?")
    kept = "/".join("" if _content_slug(s) else s for s in path.split("/")) + sep + query
    return bool(ACTIONS.search(kept) or ACTION_WORDS.search(kept))


def is_fragment(profile: Profile, path: str) -> bool:
    return any(re.search(p, path) for p in profile.fragments)
