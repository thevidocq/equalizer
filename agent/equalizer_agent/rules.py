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
# Links that change something: never followed by a read-only crawler.
ACTIONS = re.compile(r"(logout|log-out|signout|sign-out|delete|remove|destroy|unsubscribe|reset|drop|purge|"
                     r"action=(del|delete|remove|logout|edit|update))", re.I)


# Where the test account logs in. These pages stay out of the map; the
# crawler opens them only to log in.
LOGIN = {"wordpress": "/wp-login.php", "joomla": "/index.php?option=com_users&view=login",
         "drupal": "/user/login", "moodle": "/login/index.php"}


def excluded(path_query: str, cms: str | None) -> str:
    """Returns why a URL is kept out of the map ('' when it is not)."""
    for p in ADMIN.get(cms or "", []):
        if re.search(p, path_query, re.I):
            return "admin"
    for cms_rules in ADMIN.values():
        for p in cms_rules:
            if re.search(p, path_query, re.I):
                return "admin"
    for p in GENERIC:
        if re.search(p, path_query, re.I):
            return "sensitive"
    return ""


def is_action(path_query: str) -> bool:
    return bool(ACTIONS.search(path_query))
