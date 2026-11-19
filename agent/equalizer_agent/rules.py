"""Fixed safety rules: what is never crawled and never mapped. They do not
depend on the AI, which can only add exclusions."""

import re

# Admin areas and tooling, by CMS, plus everyone's usual suspects.
ADMIN = {
    "wordpress": [r"^/wp-admin(/|$)", r"^/wp-login\.php", r"^/xmlrpc\.php", r"^/wp-json(/|$)", r"^/wp-cron\.php",
                  r"^/wp-signup\.php", r"^/wp-activate\.php", r"[?&]author=\d", r"^/author/"],
    "joomla": [r"^/administrator(/|$)", r"^/api(/|$)", r"^/installation(/|$)", r"[?&]option=com_(users|login|config|installer)"],
    "drupal": [r"^/admin(/|$)", r"^/user(/|$)", r"^/jsonapi(/|$)", r"^/(core/)?install\.php", r"^/update\.php", r"^/batch(/|$)"],
    "moodle": [r"^/admin(/|$)", r"^/webservice(/|$)", r"^/login/token\.php", r"^/install\.php"],
}
GENERIC = [
    r"^/(phpmyadmin|pma|adminer|cpanel|plesk|webmail|server-status|server-info|cgi-bin)(/|$)",
    r"(^|/)\.(git|svn|hg|env|htaccess|htpasswd|ds_store)", r"\.(bak|old|orig|swp|sql|tar|tgz|gz|zip|rar|7z|log)$",
    r"(^|/)(phpinfo|info|test|debug|install|setup|config|configuration|settings)\.php$",
    r"^/(backup|backups|dump|private|tmp|temp|vendor|node_modules)(/|$)",
]
# Never opened, not even by a test account: pages that run something on a
# plain GET (scheduled jobs, installers, updates) or hand out the site's
# data in bulk (exports).
NEVER = [
    r"^/wp-cron\.php", r"^/xmlrpc\.php", r"^/wp-admin/(install|upgrade|update|update-core|setup-config|"
    r"plugin-install|theme-install|plugin-editor|theme-editor|export|import|tools|site-health|privacy-tools|"
    r"export-personal-data|erase-personal-data|post-new|media-new|user-new|link-add|press-this)\.php",
    r"^/wp-admin/maint/",
    r"^/installation(/|$)", r"^/administrator/index\.php\?.*option=com_(installer|joomlaupdate|config|admin)",
    r"^/(core/)?install\.php", r"^/update\.php", r"^/batch(/|$)", r"^/cron(/|\.php|$)",
    r"^/admin/(reports/updates|modules/(uninstall|update)|config/development|appearance/install|people/cancel)",
    r"^/install\.php", r"^/admin/cron\.php",
]
# Administrations that write to their database when a page is only viewed
# (WordPress: the dashboard saves a Quick Draft, the themes screens save
# global styles, the privacy screens save suggested texts). Found by the lab
# study (lab/e2e/tests/test_06_study.py); their administration stays closed
# to the agent even with a test account.
WRITES_ON_VIEW = {"wordpress"}
# Links that change something: never followed by a read-only crawler.
ACTIONS = re.compile(r"(logout|log-out|signout|sign-out|delete|remove|destroy|unsubscribe|reset|drop|purge|"
                     r"action=(del|delete|remove|logout|edit|update))", re.I)
# More of them, as whole words (a slug such as /running-tips/ is not "run"),
# and one-time tokens: a link carrying one acts on the site.
ACTION_WORDS = re.compile(
    r"(^|[/?&=_.\-])(trash|untrash|spam|unspam|approve|unapprove|activate|deactivate|enable|disable|install|"
    r"uninstall|upgrade|update|publish|unpublish|archive|checkin|checkout|lock|unlock|block|unblock|cancel|empty|"
    r"restore|duplicate|clone|export|import|flush|clear|rebuild|run|execute|toggle|switch|impersonate|"
    r"login_as|revoke|regenerate|resend|confirm|send)($|[/?&=_.\-])", re.I)
TOKENS = re.compile(r"[?&](_wpnonce|nonce|_nonce|token|_token|csrf|csrf_token|form_token|task)=|[?&][0-9a-f]{32}=1|"
                    r"[?&]layout=edit", re.I)  # Joomla checks an item out when its edit form opens


# Where a test account's pass starts in the administration (it is rarely
# linked from the site's pages), and the CMSs whose administration has a
# login of its own (Joomla: the site's session does not open it).
ADMIN_HOME = {"drupal": ["/admin", "/admin/content", "/admin/structure", "/admin/people"],
              "joomla": ["/administrator/index.php"], "moodle": ["/admin/index.php"]}
ADMIN_LOGIN = {"joomla": "/administrator/index.php"}

# Where the test account logs in. These pages stay out of the map; the
# crawler opens them only to log in.
LOGIN = {"wordpress": "/wp-login.php", "joomla": "/index.php?option=com_users&view=login",
         "drupal": "/user/login", "moodle": "/login/index.php"}


def excluded(path_query: str, cms: str | None, logged_in: bool = False) -> str:
    """Returns why a URL is not opened ('' when it is). A test account's
    pass opens the administration, read-only; never what NEVER lists."""
    for p in NEVER:
        if re.search(p, path_query, re.I):
            return "never"
    for p in GENERIC:
        if re.search(p, path_query, re.I):
            return "sensitive"
    if logged_in and cms not in WRITES_ON_VIEW:
        return ""
    for p in ADMIN.get(cms or "", []):
        if re.search(p, path_query, re.I):
            return "admin"
    for cms_rules in ADMIN.values():
        for p in cms_rules:
            if re.search(p, path_query, re.I):
                return "admin"
    return ""


def is_action(path_query: str) -> bool:
    return bool(ACTIONS.search(path_query) or ACTION_WORDS.search(path_query) or TOKENS.search(path_query))
