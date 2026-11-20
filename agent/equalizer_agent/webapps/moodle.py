"""Moodle."""
import re

from .base import Profile

PROFILE = Profile(
    name="moodle", title="Moodle",
    generator=re.compile(r"moodle\s*([\d.]+)?", re.I),
    paths=re.compile(r"/(theme/yui_combo\.php|lib/javascript\.php|pluginfile\.php|theme/image\.php)"),
    components=re.compile(r"/(mod|blocks|theme)/([a-z0-9_]+)/", re.I),
    login="/login/index.php",
    admin=(r"^/admin(/|$)", r"^/webservice(/|$)", r"^/login/token\.php", r"^/install\.php", r"^/user/(editadvanced|profile)\.php"),
    admin_home=("/admin/search.php",),
    # /admin/index.php runs pending upgrades when an administrator opens it
    never=(r"^/admin/cron\.php", r"^/admin/index\.php", r"^/admin/tool/", r"^/backup/", r"^/course/delete\.php"),
    note="Administration closed until the lab proves that viewing it changes nothing.",
)
