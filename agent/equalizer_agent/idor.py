"""IDOR protection for JSON APIs: a list API and a detail API that share an
id get response.issues on the list and owned_by_user on the detail, so a
user opens only the objects the list showed them. Only proposed: a person
reviews the draft (the review aid flags issues_added)."""


def _segs(path: str) -> list:
    return [s for s in path.strip("/").split("/") if s]


def _detail_param(list_path: str, detail_path: str):
    """The placeholder of detail_path when it is list_path plus one {param}."""
    a, b = _segs(list_path), _segs(detail_path)
    if len(b) != len(a) + 1 or b[:-1] != a:
        return None
    last = b[-1]
    return last[1:-1] if last.startswith("{") and last.endswith("}") and len(last) > 2 else None


def _id_field(fields: dict, param: str, ptype: str):
    for name in (param, "id"):
        if (fields.get(name) or {}).get("type") == ptype:
            return name
    return None


def propose(doc: dict) -> list:
    """Adds the issues and rules to doc; returns what was proposed."""
    apis = [a for a in doc.get("api", []) if a.get("method") == "GET"]
    out = []
    for lst in apis:
        resp = lst.get("response") or {}
        if not resp.get("list"):
            continue
        for det in apis:
            param = _detail_param(lst.get("path", ""), det.get("path", ""))
            if not param:
                continue
            ptype = ((det.get("params") or {}).get(param) or {}).get("type")
            field = _id_field(resp.get("fields") or {}, param, ptype) if ptype else None
            if not field:
                continue
            target = f"params.{param}"
            issues = resp.setdefault("issues", {})
            if field in issues:
                continue
            authz = det.setdefault("authz", {})
            added = [r for r in det.get("roles", []) if r not in authz]
            for role in added:
                authz[role] = {"check": "owned_by_user", "target": target}
            if not any(r.get("check") == "owned_by_user" and r.get("target") == target for r in authz.values()):
                if not authz:
                    det.pop("authz")
                if not issues:
                    resp.pop("issues")
                continue  # a person chose another rule for every role
            issues[field] = f"{det['id']}.{target}"
            out.append({"list": lst["id"], "detail": det["id"], "field": field, "roles": added})
    return out
