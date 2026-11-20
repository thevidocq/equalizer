"""MediaWiki."""
import re

from .base import Profile

PROFILE = Profile(
    name="mediawiki", title="MediaWiki",
    generator=re.compile(r"mediawiki\s*([\d.]+)?", re.I),
    paths=re.compile(r"/load\.php\?|/index\.php\?title=|/skins/(Vector|MonoBook|Timeless|MinervaNeue)/"),
    components=re.compile(r"/(extensions|skins)/([A-Za-z0-9_]+)/", re.I),
    login="/index.php?title=Special:UserLogin",
    admin=(r"[?&/]title=Special:|/Special:", r"^/api\.php", r"^/rest\.php", r"^/mw-config(/|$)"),
    # edits, moves, protections, rollbacks and purges are links with an action
    never=(r"[?&]action=(edit|submit|delete|protect|unprotect|purge|rollback|markpatrolled|watch|unwatch|revert|"
           r"move|editredlink)\b", r"Special:(UserLogout|Export|Import|Block|Unblock|DeletePage|MovePage|Userrights|"
           r"Undelete|MergeHistory|RevisionDelete|ChangeContentModel|EmailUser)", r"^/mw-config(/|$)"),
    tested="Lab (SQLite): pages, special pages, login; studied with the administrator logged in, database unchanged; 38 of 38 pages identical through the draft.",
    note="Special pages closed until the lab proves that viewing them changes nothing; edit links are never opened.",
)
