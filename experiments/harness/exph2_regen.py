"""EXP-H2 arm F2 — full regeneration of the 10 primary EXP-H analyses through the UNCHANGED engine.

Body = results/exp-h/report-N/request.json verbatim, except
  * name  -> "EXP-H2 F2 report N"
  * llm_target.generation = {"max_tokens": {"uint": 16000}}   (cost-only cap, D24; the engine maps it
    to the provider's max_completion_tokens: modules/simulations/mapper.go:37-41 -> openai.go:363-366)
POST /simulations/:id/analyses (202) -> poll GET …/analyses/:aid until completed|failed ->
save report.json (full GET body) + analysis.pdf (GridFS) + request.json.

Usage:
  python harness/exph2_regen.py --dry-run            # write the 10 request bodies, no POST
  python harness/exph2_regen.py --run --report N     # one live report (called by exph2_run.py)
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import sys
import time
from datetime import datetime
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from exph2_common import (GENERATION_CAP, OUT, PRIMARY, SIBLING_HARNESS, console_utf8,  # noqa: E402
                          cost_usd, load_primary, write_json)

sys.path.insert(0, str(SIBLING_HARNESS))
from intellisim_client import IntelliSimClient  # noqa: E402

POLL_S = 3.0
POLL_TIMEOUT_S = 15 * 60


def f2_body(n: int) -> tuple[str, dict]:
    req, orig = load_primary(n)
    body = copy.deepcopy(req)
    body["name"] = f"EXP-H2 F2 report {n}"
    body["llm_target"]["generation"] = copy.deepcopy(GENERATION_CAP)
    return orig["simulation_id"], body


def _ts(s: str) -> float:
    return datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()


def run_report(n: int, c: IntelliSimClient) -> dict:
    sim_id, body = f2_body(n)
    d = OUT / f"f2-{n}"
    write_json(d / "request.json", body)
    created = c._request("POST", f"/simulations/{sim_id}/analyses", json=body).json()
    aid = created["id"]
    t0 = time.monotonic()
    while True:
        a = c.get_analysis(sim_id, aid)
        if a.get("status") in ("completed", "failed"):
            break
        if time.monotonic() - t0 > POLL_TIMEOUT_S:
            raise SystemExit(f"F2 report {n}: analysis {aid} still {a.get('status')} after {POLL_TIMEOUT_S}s")
        time.sleep(POLL_S)
    write_json(d / "report.json", a)
    if a.get("status") == "completed" and a.get("file"):
        c.download_analysis_pdf(sim_id, aid, str(d / "analysis.pdf"))
    tu = (a.get("token_usage") or {}).get("total") or {}
    wall = (_ts(a["finished_at"]) - _ts(a["created_at"])) if a.get("finished_at") and a.get("created_at") else None
    return {"arm": "F2", "report_no": n, "analysis_id": aid, "status": a.get("status"),
            "calls": tu.get("calls"), "requests": a.get("requests"), "latency_ms": tu.get("latency_ms"),
            "wall_clock_s": round(wall, 1) if wall is not None else None,
            "warnings": len((a.get("report") or {}).get("warnings") or a.get("warnings") or []),
            "resolved_models": [m.get("model_uid") for m in (a.get("token_usage") or {}).get("by_model", [])],
            "llm_target_echo": a.get("llm_target"),
            "cost_usd_engine": round(cost_usd("o4-mini", tu.get("prompt_tokens", 0), tu.get("completion_tokens", 0)), 6)}


def main() -> int:
    console_utf8()
    ap = argparse.ArgumentParser()
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--dry-run", action="store_true")
    g.add_argument("--run", action="store_true")
    ap.add_argument("--report", type=int)
    a = ap.parse_args()
    if a.dry_run:
        hashes = {}
        for n in PRIMARY:
            sim_id, body = f2_body(n)
            write_json(OUT / "dryrun" / f"f2-{n}" / "request.json", {"simulation_id": sim_id, "body": body})
            hashes[f"f2-{n}"] = hashlib.sha256(json.dumps(body, ensure_ascii=False, sort_keys=True).encode("utf-8")).hexdigest()
            print(f"f2-{n}: {sim_id} lang={body['language']} target={body['llm_target']}")
        write_json(OUT / "dryrun" / "f2-hashes.json", hashes)
        return 0
    print(json.dumps(run_report(a.report, IntelliSimClient()), ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
