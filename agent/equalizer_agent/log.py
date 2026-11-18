"""JSON logs (schema/events/log.schema.json). Never secrets or values seen
on the site: only paths, counts and reasons."""

import json
import sys
from datetime import datetime, timezone

LEVELS = {"debug": 10, "info": 20, "warn": 30, "error": 40}


class Logger:
    def __init__(self, instance: str, level: str = "info", out=sys.stdout):
        self.instance, self.min, self.out = instance, LEVELS.get(level, 20), out

    def _emit(self, level: str, msg: str, **detail):
        if LEVELS[level] < self.min:
            return
        rec = {"ts": datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z"),
               "level": level, "msg": msg, "service": "agent", "instance": self.instance}
        rid = detail.pop("request_id", None)
        if rid:
            rec["request_id"] = rid
        err = detail.pop("error", None)
        if err:
            rec["error"] = str(err)[:300]
        if detail:
            rec["detail"] = detail
        self.out.write(json.dumps(rec, ensure_ascii=False) + "\n")
        self.out.flush()

    def debug(self, msg, **d): self._emit("debug", msg, **d)
    def info(self, msg, **d): self._emit("info", msg, **d)
    def warn(self, msg, **d): self._emit("warn", msg, **d)
    def error(self, msg, **d): self._emit("error", msg, **d)
