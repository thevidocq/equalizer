"""The attack set tried on every parameter and field of a version, on the
gateway only (dry-run: nothing reaches the legacy site). An input protects
the site when the gateway refuses every value; the categories it lets
through become its "still open" reason (docs/agent.md, step 5)."""

SET = {
    "accepts_sql": ["' OR '1'='1", "1; DROP TABLE users--", "\" OR 1=1 -- ", "1 UNION SELECT NULL--"],
    "accepts_markup": ["<script>alert(1)</script>", "\"><img src=x onerror=alert(1)>", "javascript:alert(1)"],
    "accepts_template": ["{{7*7}}", "${7*7}", "<?php system('id'); ?>"],
    "accepts_path": ["../../../../etc/passwd", "..\\..\\windows\\win.ini", "a\x00b"],
    "accepts_shell": ["; cat /etc/passwd", "| id", "$(id)", "`id`"],
    "other": ["9" * 40, "A" * 5000, "x\r\nX-Injected: 1"],
}


def values():
    """(reason, value) for every attack."""
    for reason, vals in SET.items():
        for v in vals:
            yield reason, v


def valid(spec: dict) -> str:
    """A value the field's own specification accepts, for the fields not
    under attack."""
    t = spec.get("type", "string")
    if t == "enum":
        return (spec.get("values") or [""])[0]
    if t in ("int", "id"):
        lo = spec.get("min", 1)
        return str(max(lo, 1) if spec.get("max", 1) >= 1 else lo)
    if t == "email":
        return "agent@example.com"
    if t == "date":
        return "2024-01-15"
    if t == "password":
        return "Agent-check-1"
    if t == "bool":
        return "1"
    return "abc"
