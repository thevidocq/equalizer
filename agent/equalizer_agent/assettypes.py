"""Which types the agent may add on its own to an asset prefix already in
the map (a template's /wp-content/uploads/, a published prefix).

A prefix is stricter than the site area: whatever type it does not list is
refused, and the gateway judges a file by its type, not its name. Adding a
type opens every file of that type under the prefix, linked or not. So only
passive types are ever added by the agent: what a browser shows or plays
without running it (images, fonts, audio, video, stylesheets, PDF and office
documents). Scripts are added only when a page loads them. Text, CSV,
archives, SQL, JSON, HTML and XML (logs, exports, backups, documents that
can carry script) never are: they stay for a person to decide.
"""

import re

# What a map can hold (schema/map/gateway-map.schema.json, asset types).
VALID = re.compile(r"^[a-z]+/[a-z0-9.+\-]+$")

_DOCUMENTS = {
    "application/pdf", "application/rtf", "application/epub+zip", "application/msword",
    "application/vnd.ms-excel", "application/vnd.ms-powerpoint", "application/vnd.ms-fontobject",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    "application/vnd.oasis.opendocument.text", "application/vnd.oasis.opendocument.spreadsheet",
    "application/vnd.oasis.opendocument.presentation", "application/font-woff", "application/x-font-woff",
    "application/manifest+json", "text/css",
}
SCRIPTS = {"application/javascript", "text/javascript", "application/x-javascript", "application/ecmascript",
            "text/ecmascript", "application/wasm"}


def passive(kind: str, loaded: bool = True) -> bool:
    """Whether the agent may add kind to a prefix on its own. loaded: a
    page loads the file (src, a stylesheet), not only links to it."""
    if not VALID.match(kind or ""):
        return False
    if kind.split("/")[0] in ("image", "font", "audio", "video"):
        return "xml" not in kind or kind == "image/svg+xml"  # SVG: sandboxed by the gateway
    if kind in SCRIPTS:
        return loaded
    return kind in _DOCUMENTS
