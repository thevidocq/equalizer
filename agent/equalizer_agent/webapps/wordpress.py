"""WordPress."""
import re

from .base import Profile

PROFILE = Profile(
    name="wordpress", title="WordPress",
    generator=re.compile(r"wordpress\s*([\d.]+)?", re.I),
    paths=re.compile(r"/wp-(content|includes)/"),
    components=re.compile(r"/wp-content/(plugins|themes)/([a-z0-9_\-]+)/[^?\s\"']*(?:\?ver=([\d.]+))?", re.I),
    login="/wp-login.php",
    admin=(r"^/wp-admin(/|$)", r"^/wp-login\.php", r"^/xmlrpc\.php", r"^/wp-json(/|$)", r"^/wp-cron\.php",
           r"^/wp-signup\.php", r"^/wp-activate\.php", r"[?&]author=\d", r"^/author/"),
    # The lab found that WordPress writes to its database when its screens
    # are only viewed (the dashboard saves a Quick Draft, the theme screens
    # global styles, the privacy screens suggested texts): closed.
    admin_read_only=False,
    never=(r"^/wp-cron\.php", r"^/xmlrpc\.php",
           r"^/wp-admin/(install|upgrade|update|update-core|setup-config|plugin-install|theme-install|plugin-editor|"
           r"theme-editor|export|import|tools|site-health|privacy-tools|export-personal-data|erase-personal-data|"
           r"post-new|media-new|user-new|link-add|press-this)\.php", r"^/wp-admin/maint/"),
    tested="Lab: public site, login, comments, search, REST API with nonce, uploads; studied with an "
           "administrator account, database unchanged.",
    note="Administration not studied: WordPress writes to its database when its screens are only viewed.",
)
