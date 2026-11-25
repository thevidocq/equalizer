"""panel-api internal endpoints used by the agent (service token)."""

import json
import urllib.error
import urllib.request


class Panel:
    def __init__(self, base: str, token: str, timeout: float = 30):
        self.base, self.token, self.timeout = base, token, timeout

    def _post(self, path: str, body=None):
        data = json.dumps(body if body is not None else {}).encode()
        req = urllib.request.Request(self.base + path, data=data, method="POST", headers={
            "Authorization": "Bearer " + self.token, "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                raw = resp.read()
                return resp.status, (json.loads(raw) if raw else None)
        except urllib.error.HTTPError as e:
            return e.code, None

    def claim(self):
        status, job = self._post("/v1/internal/discoveries/claim")
        return job if status == 200 else None

    def claim_review(self):
        status, job = self._post("/v1/internal/reviews/claim")
        return job if status == 200 else None

    def review_result(self, job_id: int, summary="", questions=None, error=None):
        body = {"summary": summary[:3000], "questions": [q[:400] for q in (questions or [])][:20]}
        if error:
            body["error"] = error[:500]
        status, _ = self._post(f"/v1/internal/reviews/{job_id}/result", body)
        return status == 204

    def progress(self, job_id: int, progress: dict):
        self._post(f"/v1/internal/discoveries/{job_id}/progress", progress)

    def result(self, job_id: int, report=None, document=None, error=None):
        body = {"report": report}
        if document is not None:
            body["document"] = document
        if error:
            body["error"] = error[:500]
        status, _ = self._post(f"/v1/internal/discoveries/{job_id}/result", body)
        return status == 204

    def claim_verification(self):
        status, job = self._post("/v1/internal/verifications/claim")
        return job if status == 200 else None

    def claim_study(self):
        status, job = self._post("/v1/internal/studies/claim")
        return job if status == 200 else None

    def study_result(self, discovery_id: int, security: dict) -> bool:
        status, _ = self._post(f"/v1/internal/studies/{discovery_id}/result", {"security": security})
        return status == 204

    def try_map(self, job_id: int, document: dict):
        """The panel signs a corrected version for tries; None when it refuses it."""
        status, signed = self._post(f"/v1/internal/verifications/{job_id}/try-map", {"document": document})
        return signed if status == 200 else None

    def verification_progress(self, job_id: int, step: str, done: int, total: int, account=None) -> None:
        """Where the verification is; a courtesy: a failure changes nothing."""
        try:
            self._post(f"/v1/internal/verifications/{job_id}/progress",
                       {"step": step, "done": min(done, total), "total": total, "account": account})
        except Exception:  # noqa: BLE001
            pass

    def verification_result(self, job_id: int, score=None, broken=None, still_open=None, error=None,
                            document=None, corrections=None, before=None):
        body = {"score": score, "broken": (broken or [])[:500], "open": (still_open or [])[:1000]}
        if error:
            body["error"] = error[:500]
        if document is not None:
            body.update(document=document, corrections=(corrections or [])[:500], before=before)
        status, _ = self._post(f"/v1/internal/verifications/{job_id}/result", body)
        return status == 204
