"""Pushes the security events of a real study (the lab's WordPress behind
the gateway, fixtures/gateway-events.jsonl) into Loki, as the blog of the
demo organisation, spread over the last twelve hours, with the labels
Vector gives them (deploy/compose/vector/vector.yaml).

    python3 scripts/screenshots/logs.py <Loki URL>"""

import json
import os
import sys
import time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))


def main(loki):
    with open(os.path.join(HERE, "fixtures", "gateway-events.jsonl")) as f:
        events = [json.loads(line) for line in f if line.strip()]
    now = time.time_ns()
    span = 12 * 3600 * 10**9
    values = []
    for n, e in enumerate(events):
        at = now - span + span * n // len(events)
        e.update(tenant="acme", app="blog", instance="gateway-1",
                 ts=time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(at / 1e9)) + ".000Z")
        values.append([str(at), json.dumps(e, separators=(",", ":"))])
    body = {"streams": [{"stream": {"service": "gateway", "stream": "security", "level": "info"}, "values": values}]}
    req = urllib.request.Request(loki.rstrip("/") + "/loki/api/v1/push", data=json.dumps(body).encode(), method="POST",
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as r:
        print("pushed", len(values), "events:", r.status)


if __name__ == "__main__":
    main(sys.argv[1])
