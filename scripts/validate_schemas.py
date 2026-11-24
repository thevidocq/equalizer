#!/usr/bin/env python3
"""Validate the contracts in schema/.

- JSON Schemas are valid schemas (draft 2020-12).
- Map examples in schema/map/examples/ conform to the map schema.
- OpenAPI documents are valid.

Usage: python3 scripts/validate_schemas.py
Exits with code 1 if any group reports errors.
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
                where = "/".join(str(p) for p in err.path) or "(root)"
                errors.append(f"{path.relative_to(ROOT)} @ {where}: {err.message}")
        else:
            print(f"ok   example  {path.relative_to(ROOT)}")
    return errors


def check_source_examples() -> list[str]:
    """The source reader's summaries in schema/source/examples/ (written by
    its tests from a fixture site) conform to the summary schema."""
    errors: list[str] = []
    validator = Draft202012Validator(load(SCHEMA_DIR / "source" / "summary.schema.json"))
    for path in sorted((SCHEMA_DIR / "source" / "examples").glob("*.json")):
        found = sorted(validator.iter_errors(load(path)), key=lambda e: list(e.path))
        if found:
            for err in found:
                where = "/".join(str(p) for p in err.path) or "(root)"
                errors.append(f"{path.relative_to(ROOT)} @ {where}: {err.message}")
        else:
            print(f"ok   example  {path.relative_to(ROOT)}")
    return errors


def check_openapi() -> list[str]:
    try:
        from openapi_spec_validator import validate
    except ImportError:
        return ["openapi-spec-validator not installed (pip install openapi-spec-validator)"]
    errors: list[str] = []
    for path in sorted((SCHEMA_DIR / "api").glob("*.y*ml")):
        try:
            validate(load(path))
            print(f"ok   openapi  {path.relative_to(ROOT)}")
        except Exception as exc:  # noqa: BLE001
            errors.append(f"{path.relative_to(ROOT)}: {exc}")
    return errors


def check_copies() -> list[str]:
    """Services that embed a contract keep an identical copy."""
    errors: list[str] = []
    copies = {
        ROOT / "panel-api" / "internal" / "mapschema" / "gateway-map.schema.json": SCHEMA_DIR / "map" / "gateway-map.schema.json",
        ROOT / "panel-api" / "internal" / "sourceschema" / "summary.schema.json": SCHEMA_DIR / "source" / "summary.schema.json",
    }
    for copy, source in copies.items():
        if not copy.exists():
            continue
        if copy.read_bytes() != source.read_bytes():
            errors.append(f"{copy.relative_to(ROOT)} differs from {source.relative_to(ROOT)}: copy it again")
        else:
            print(f"ok   copy     {copy.relative_to(ROOT)}")
    return errors


def main() -> int:
    errors = check_json_schemas() + check_map_examples() + check_source_examples() + check_openapi() + check_copies()
    if errors:
        print("\nERRORS:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1
    print("\nAll contracts are valid.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
