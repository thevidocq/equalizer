"""Emulates openapi-typescript 7 (default options) for panel-api.yaml, where
the npm registry is unreachable:

    python3 scripts/openapi_ts.py schema/api/panel-api.yaml > panel-ui/src/shared/api/schema.d.ts

It covers the OpenAPI features panel-api.yaml uses and reproduces the
generator's output byte for byte; CI's check:api runs the real one, so a
feature it does not cover shows there."""
import json
import sys

import yaml

IND = "    "
IN_COMPONENTS = [False]


class Loader(yaml.SafeLoader):
    """YAML 1.2 booleans: only true and false (off, yes, no are strings)."""


Loader.yaml_implicit_resolvers = {k: [(t, r) for t, r in v if t != "tag:yaml.org,2002:bool"]
                                  for k, v in yaml.SafeLoader.yaml_implicit_resolvers.items()}
import re as _re
Loader.add_implicit_resolver("tag:yaml.org,2002:bool", _re.compile(r"^(?:true|false)$"), list("tf"))
METHODS = ["get", "put", "post", "delete", "options", "head", "patch", "trace"]


def comment(lines, ind):
    lines = [l for l in lines if l is not None]
    if not lines:
        return ""
    if len(lines) == 1 and "\n" not in lines[0]:
        return ind + "/** " + lines[0].replace("*/", "*\\/") + " */\n"
    out = ind + "/**\n"
    for l in lines:
        for i, part in enumerate(l.split("\n")):
            text = ("    " + part if i else part).replace("*/", "*\\/")
            out += (ind + " * " + text).rstrip(" ") + "\n"
    return out + ind + " */\n"


def schema_comment(s):
    lines = []
    if not isinstance(s, dict):
        return lines
    if s.get("deprecated"):
        lines.append("@deprecated")
    if "format" in s:
        lines.append("Format: " + s["format"])
    if "description" in s:
        lines.append("@description " + str(s["description"]).strip())
    if "default" in s:
        lines.append("@default " + json.dumps(s["default"], ensure_ascii=False) if not isinstance(s["default"], str) else "@default " + s["default"])
    if "const" in s:
        lines.append("@constant")
    if "enum" in s:
        t = s.get("type")
        if isinstance(t, list):
            t = "|".join(t)
        lines.append("@enum {%s}" % (t or "unknown"))
    for ex in s.get("examples") or []:
        lines.append("@example " + (ex if isinstance(ex, str) else json.dumps(ex)))
    if "example" in s:
        lines.append("@example " + (s["example"] if isinstance(s["example"], str) else json.dumps(s["example"])))
    return lines


def lit(v):
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return json.dumps(v)
    return json.dumps(v, ensure_ascii=False)


def ts(s, ind):
    """TypeScript for a schema, at indentation ind (of the line it sits on)."""
    if s is True or s == {}:
        return "unknown"
    if "$ref" in s:
        ref = s["$ref"].split("/")
        return 'components["%s"]["%s"]' % (ref[-2], ref[-1])
    for k in ("oneOf", "anyOf"):
        if k in s:
            parts = [ts(x, ind) for x in s[k]]
            base = " | ".join(parts)
            return base
    if "allOf" in s:
        return " & ".join(ts(x, ind) for x in s["allOf"])
    t = s.get("type")
    nullable = False
    if isinstance(t, list):
        nullable = "null" in t
        rest = [x for x in t if x != "null"]
        t = rest[0] if len(rest) == 1 else rest
    if "const" in s:
        out = lit(s["const"])
    elif "enum" in s:
        vals = list(s["enum"])
        if nullable and None not in vals:
            pass
        out = " | ".join(lit(v) for v in vals)
        return out + (" | null" if nullable and None not in vals else "")
    elif isinstance(t, list):
        out = " | ".join(ts(dict(s, type=x), ind) for x in t)
    elif t == "string":
        out = "string"
    elif t in ("integer", "number"):
        out = "number"
    elif t == "boolean":
        out = "boolean"
    elif t == "null":
        out = "null"
    elif t == "array":
        items = s.get("items", {})
        inner = ts(items, ind)
        if _union(items):
            inner = "(" + inner + ")"
        out = inner + "[]"
    elif t == "object" or "properties" in s or "additionalProperties" in s:
        out = obj(s, ind)
    else:
        out = "unknown"
    return out + (" | null" if nullable else "")


def _union(s):
    if not isinstance(s, dict) or "$ref" in s:
        return False
    if "oneOf" in s or "anyOf" in s or "allOf" in s:
        return True
    t = s.get("type")
    if isinstance(t, list) and len(t) > 1:
        return True
    return "enum" in s and len(s["enum"]) > 1


def obj(s, ind):
    props = s.get("properties") or {}
    ap = s.get("additionalProperties")
    req = set(s.get("required") or [])
    if not props and (ap is None or ap is False):
        return "Record<string, never>"
    if not props and ap is True:
        return "{\n" + ind + IND + "[key: string]: unknown;\n" + ind + "}"
    if not props and isinstance(ap, dict):
        if ap == {}:
            return "{\n" + ind + IND + "[key: string]: unknown;\n" + ind + "}"
        return "{\n" + ind + IND + "[key: string]: " + ts(ap, ind + IND) + ";\n" + ind + "}"
    out = "{\n"
    for name, p in props.items():
        out += comment(schema_comment(p), ind + IND)
        key = name if name.isidentifier() else json.dumps(name)
        opt = "" if name in req or (IN_COMPONENTS[0] and isinstance(p, dict) and "default" in p) else "?"
        if isinstance(p, dict) and p.get("writeOnly") is not None:
            pass
        ro = "readonly " if isinstance(p, dict) and p.get("readOnly") else ""
        out += ind + IND + ro + key + opt + ": " + ts(p, ind + IND) + ";\n"
    out += ind + "}"
    if ap is True or ap == {}:
        out += " & {\n" + ind + IND + "[key: string]: unknown;\n" + ind + "}"
    elif isinstance(ap, dict):
        out += " & {\n" + ind + IND + "[key: string]: " + ts(ap, ind + IND) + ";\n" + ind + "}"
    return out


def params_block(params, comps, ind):
    groups = {"query": [], "header": [], "path": [], "cookie": []}
    for p in params:
        if "$ref" in p:
            name = p["$ref"].split("/")[-1]
            real = comps["parameters"][name]
            groups[real["in"]].append((real, 'components["parameters"]["%s"]' % name))
        else:
            groups[p["in"]].append((p, None))
    out = ind + "parameters: {\n"
    for g in ("query", "header", "path", "cookie"):
        items = groups[g]
        if not items:
            out += ind + IND + g + "?: never;\n"
            continue
        allopt = all(not p.get("required") for p, _ in items)
        out += ind + IND + g + ("?" if allopt else "") + ": {\n"
        for p, ref in items:
            out += comment(["@description " + p["description"].strip()] if p.get("description") else [], ind + IND + IND)
            typ = ref or ts(p.get("schema", {}), ind + IND + IND)
            out += ind + IND + IND + p["name"] + ("" if p.get("required") else "?") + ": " + typ + ";\n"
        out += ind + IND + "};\n"
    return out + ind + "};\n"


def response(r, ind):
    if "$ref" in r:
        ref = r["$ref"].split("/")
        return 'components["%s"]["%s"]' % (ref[-2], ref[-1])
    out = "{\n" + ind + IND + "headers: {\n" + ind + IND + IND + "[name: string]: unknown;\n" + ind + IND + "};\n"
    content = r.get("content")
    if not content:
        out += ind + IND + "content?: never;\n"
    else:
        out += ind + IND + "content: {\n"
        for mt, m in content.items():
            out += ind + IND + IND + json.dumps(mt) + ": " + ts(m.get("schema", {}), ind + IND + IND) + ";\n"
        out += ind + IND + "};\n"
    return out + ind + "}"


def operation(op, path_params, comps, ind):
    out = "{\n"
    params = list(path_params) + list(op.get("parameters") or [])
    out += params_block(params, comps, ind + IND)
    rb = op.get("requestBody")
    if rb is None:
        out += ind + IND + "requestBody?: never;\n"
    else:
        out += ind + IND + "requestBody" + ("" if rb.get("required") else "?") + ": {\n"
        out += ind + IND + IND + "content: {\n"
        for mt, m in rb["content"].items():
            out += ind + IND + IND + IND + json.dumps(mt) + ": " + ts(m.get("schema", {}), ind + IND + IND + IND) + ";\n"
        out += ind + IND + IND + "};\n" + ind + IND + "};\n"
    out += ind + IND + "responses: {\n"
    for code, r in op.get("responses", {}).items():
        if "$ref" not in r:
            out += comment(["@description " + r["description"].strip()] if r.get("description") else [], ind + IND + IND)
        c = str(code)
        key = c if c.isdigit() else json.dumps(c)
        out += ind + IND + IND + key + ": " + response(r, ind + IND + IND) + ";\n"
    out += ind + IND + "};\n"
    return out + ind + "}"


def op_comment(op, ind):
    lines = []
    if op.get("deprecated"):
        lines.append("@deprecated")
    if op.get("summary") and op.get("description"):
        lines.append("@summary " + op["summary"].strip())
        lines.append("@description " + op["description"].strip())
    elif op.get("summary"):
        lines.append(op["summary"].strip())
    elif op.get("description"):
        lines.append("@description " + op["description"].strip())
    return comment(lines, ind)


def main(path):
    spec = yaml.load(open(path), Loader=Loader)
    comps = spec.get("components", {})
    out = "/**\n * This file was auto-generated by openapi-typescript.\n * Do not make direct changes to the file.\n */\n\n"
    out += "export interface paths {\n"
    for p, item in spec["paths"].items():
        out += IND + json.dumps(p) + ": {\n"
        out += params_block(item.get("parameters") or [], comps, IND + IND)
        for m in METHODS:
            if m in item:
                out += op_comment(item[m], IND + IND)
                out += IND + IND + m + ": " + operation(item[m], item.get("parameters") or [], comps, IND + IND) + ";\n"
            else:
                out += IND + IND + m + "?: never;\n"
        out += IND + "};\n"
    out += "}\nexport type webhooks = Record<string, never>;\nexport interface components {\n"
    IN_COMPONENTS[0] = True
    out += IND + "schemas: {\n"
    for n, s in (comps.get("schemas") or {}).items():
        out += comment(schema_comment(s), IND + IND)
        out += IND + IND + n + ": " + ts(s, IND + IND) + ";\n"
    out += IND + "};\n"
    out += IND + "responses: {\n"
    for n, r in (comps.get("responses") or {}).items():
        out += comment(["@description " + r["description"].strip()] if r.get("description") else [], IND + IND)
        out += IND + IND + n + ": " + response(r, IND + IND) + ";\n"
    out += IND + "};\n"
    out += IND + "parameters: {\n"
    for n, p in (comps.get("parameters") or {}).items():
        out += comment(["@description " + p["description"].strip()] if p.get("description") else [], IND + IND)
        out += IND + IND + n + ": " + ts(p.get("schema", {}), IND + IND) + ";\n"
    out += IND + "};\n" + IND + "requestBodies: never;\n" + IND + "headers: never;\n" + IND + "pathItems: never;\n}\n"
    out += "export type $defs = Record<string, never>;\nexport type operations = Record<string, never>;\n"
    sys.stdout.write(out)


main(sys.argv[1])
