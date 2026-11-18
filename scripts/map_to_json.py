#!/usr/bin/env python3
"""Convert a YAML map to JSON after validating it against the schema.

Usage: python3 scripts/map_to_json.py schema/map/examples/permits-portal.yaml > map.json

In the final product panel-api does this; the script exists for the prototype.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parent.parent
SCHEMA = ROOT / "schema" / "map" / "gateway-map.schema.json"


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    doc = yaml.safe_load(Path(sys.argv[1]).read_text(encoding="utf-8"))
    validator = Draft202012Validator(json.loads(SCHEMA.read_text(encoding="utf-8")))
    errors = sorted(validator.iter_errors(doc), key=lambda e: list(e.path))
    if errors:
        for err in errors:
            where = "/".join(str(p) for p in err.path) or "(root)"
            print(f"error @ {where}: {err.message}", file=sys.stderr)
        return 1
    json.dump(doc, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
