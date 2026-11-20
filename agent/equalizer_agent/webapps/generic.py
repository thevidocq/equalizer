"""Custom sites and anything not recognised."""
from .base import Profile

PROFILE = Profile(
    name="generic", title="Custom site",
    admin=(r"^/(admin|administrator|backend|manager|console|controlpanel)(/|$)",),
    tested="Lab: a custom PHP intranet in ISO-8859-1 (sessions, attachments), served in plain http; studied "
           "with a test account, database unchanged.",
    note="Recognised by nothing in particular: the shared rules apply (no action links, no sensitive files); "
         "a usual administration path stays closed.",
)
