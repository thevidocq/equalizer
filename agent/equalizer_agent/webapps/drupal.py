"""Drupal."""
import re

from .base import Profile

PROFILE = Profile(
    name="drupal", title="Drupal",
    generator=re.compile(r"drupal\s*([\d.]+)?", re.I),
    paths=re.compile(r"/(sites/default/files|core/misc|misc/drupal\.js)"),
    components=re.compile(r"/(modules|themes)/(?:contrib/)?([a-z0-9_]+)/", re.I),
    login="/user/login",
    admin=(r"^/admin(/|$)", r"^/user(/|$)", r"^/jsonapi(/|$)", r"^/(core/)?install\.php", r"^/update\.php", r"^/batch(/|$)"),
    admin_home=("/admin", "/admin/content", "/admin/structure", "/admin/people"),
    admin_read_only=True,
    never=(r"^/(core/)?install\.php", r"^/update\.php", r"^/batch(/|$)", r"^/cron(/|\.php|$)",
           r"^/admin/(reports/updates|modules/(uninstall|update)|config/development|appearance/install|people/cancel)"),
    tested="Lab: nodes, search, login; studied with an administrator account, database unchanged.",
)
