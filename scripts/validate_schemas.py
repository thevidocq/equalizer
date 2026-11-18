#!/usr/bin/env python3
"""Valida i contratti in schema/.

- I JSON Schema sono schemi validi (draft 2020-12).
- Gli esempi di mappa in schema/map/examples/ rispettano lo schema della mappa.
- Le OpenAPI sono valide.

Uso: python3 scripts/validate_schemas.py
Esce con codice 1 al primo gruppo di errori.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_DIR = ROOT / "schema"


def load(path: Path):
    text = path.read_text(encoding="utf-8")
    if path.suffix in (".yaml", ".yml"):
        return yaml.safe_load(text)
    return json.loads(text)


def check_json_schemas() -> list[str]:
    errors: list[str] = []
    for path in sorted(SCHEMA_DIR.rglob("*.schema.json")):
        try:
            Draft202012Validator.check_schema(load(path))
            print(f"ok   schema   {path.relative_to(ROOT)}")
        except Exception as exc:  # noqa: BLE001
            errors.append(f"{path.relative_to(ROOT)}: {exc}")
    return errors


def check_map_examples() -> list[str]:
    errors: list[str] = []
    validator = Draft202012Validator(load(SCHEMA_DIR / "map" / "gateway-map.schema.json"))
    for path in sorted((SCHEMA_DIR / "map" / "examples").glob("*.y*ml")):
        found = sorted(validator.iter_errors(load(path)), key=lambda e: list(e.path))
        if found:
            for err in found:
                where = "/".join(str(p) for p in err.path) or "(radice)"
                errors.append(f"{path.relative_to(ROOT)} @ {where}: {err.message}")
        else:
            print(f"ok   esempio  {path.relative_to(ROOT)}")
    return errors


def check_openapi() -> list[str]:
    try:
        from openapi_spec_validator import validate
    except ImportError:
        return ["openapi-spec-validator non installato (pip install openapi-spec-validator)"]
    errors: list[str] = []
    for path in sorted((SCHEMA_DIR / "api").glob("*.y*ml")):
        try:
            validate(load(path))
            print(f"ok   openapi  {path.relative_to(ROOT)}")
        except Exception as exc:  # noqa: BLE001
            errors.append(f"{path.relative_to(ROOT)}: {exc}")
    return errors


def main() -> int:
    errors = check_json_schemas() + check_map_examples() + check_openapi()
    if errors:
        print("\nERRORI:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1
    print("\nTutti i contratti sono validi.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
