"""SMTP sink for the local lab: every message is saved in .run/mail as an
.eml file, nothing is delivered. Never outside the lab."""
import asyncio
import pathlib
import sys
import time

from aiosmtpd.controller import Controller

OUT = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else ".run/mail")
OUT.mkdir(parents=True, exist_ok=True)


class Sink:
    async def handle_DATA(self, server, session, envelope):  # noqa: N802 (aiosmtpd API)
        name = f"{time.time_ns()}.eml"
        rcpt = ",".join(envelope.rcpt_tos)
        (OUT / name).write_bytes(f"X-Lab-To: {rcpt}\r\n".encode() + envelope.content)
        return "250 stored"


if __name__ == "__main__":
    c = Controller(Sink(), hostname="127.0.0.1", port=2525)
    c.start()
    print("lab SMTP sink on 127.0.0.1:2525 ->", OUT, flush=True)
    asyncio.get_event_loop().run_forever()
