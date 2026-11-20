"""Joomla."""
import re

from .base import Profile

PROFILE = Profile(
    name="joomla", title="Joomla",
    generator=re.compile(r"joomla!?\s*-?\s*(?:open source content management\s*)?([\d.]+)?", re.I),
    paths=re.compile(r"/(media/jui|media/system|components/com_)"),
    components=re.compile(r"/(components|modules|plugins)/(com_[a-z0-9_]+|mod_[a-z0-9_]+)", re.I),
    login="/index.php?option=com_users&view=login",
    admin=(r"^/administrator(/|$)", r"^/api(/|$)", r"^/installation(/|$)", r"[?&]option=com_(users|login|config|installer)"),
    admin_home=("/administrator/index.php",),
    admin_login="/administrator/index.php",  # the site's session does not open the administration
    admin_read_only=True,
    never=(r"^/installation(/|$)", r"^/administrator/index\.php\?.*option=com_(installer|joomlaupdate|config|admin)"),
    tested="Lab (PostgreSQL): articles, search, login; studied with an administrator account through its own "
           "login, database unchanged.",
)
