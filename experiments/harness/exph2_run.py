"""EXP-H2 orchestrator — pilot (gate G2) and battery (gate G3), protocol 37 §6–§7.

  python harness/exph2_run.py --pilot      # A-1 then F2-1, then evaluate G2 and stop
  python harness/exph2_run.py --battery    # reports 2..10: odd n -> A then F2, even n -> F2 then A;
                                           # G3 checked after every report
Appends every report's summary to results/exp-h2/manifest.json (idempotent per arm/report:
an existing report.json is never regenerated — protocol 37: no regeneration).
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from exph2_common import (G2_MAX_COST_PER_REPORT, G3_MAX_CUMULATIVE_COST, G3_MAX_FAILED_ATTEMPTS,  # noqa: E402
                          GENERATION_CAP, OUT, SIBLING_HARNESS, console_utf8, utc_now, write_json)
import exph2_ablate  # noqa: E402
import exph2_regen  # noqa: E402

sys.path.insert(0, str(SIBLING_HARNESS))
from intellisim_client import IntelliSimClient  # noqa: E402

MANIFEST = OUT / "manifest.json"
SUSPECT_GAP_S = 30.0   # wall-clock minus summed latency above this => a hidden failed attempt (cf. EXP-H report 2)


def load_manifest() -> dict:
    if MANIFEST.exists():
        return json.loads(MANIFEST.read_text(encoding="utf-8"))
    return {"experiment": "EXP-H2", "protocol": "experiments/37-exph2-regeneration-ablation-protocol.md",
            "engine": "intellisim.core dev 5072b5f (unchanged; analysis code = EXP-H engine 6bb5bd9)",
            "provider": "intellisim.openai.llm.provider dev 777aeca (unchanged)", "rows": []}


def summarize_a(res: dict) -> dict:
    return {"arm": "A", "report_no": res["report_no"], "status": res["status"],
            "calls": res["token_usage"]["total"]["calls"], "requests": res["requests"],
            "http_attempts": res["http_attempts"], "failed_attempts": res["failed_attempts"],
            "wall_clock_s": round(res["wall_clock_ms"] / 1000, 1), "resolved_models": res["token_usage"]["by_model"],
            "warnings": len(res["warnings"]), "cost_usd": res["cost_usd_engine_equivalent"],
            "created_at": res["created_at"], "finished_at": res["finished_at"]}


def summarize_f2(row: dict) -> dict:
    gap = (row["wall_clock_s"] - (row["latency_ms"] or 0) / 1000) if row["wall_clock_s"] is not None else None
    row = dict(row)
    row["cost_usd"] = row.pop("cost_usd_engine")
    row["suspected_hidden_failed_attempts"] = int(gap is not None and gap > SUSPECT_GAP_S)
    row["wall_minus_latency_s"] = round(gap, 1) if gap is not None else None
    return row


def do(arm: str, n: int, c: IntelliSimClient, man: dict) -> dict:
    if any(r["arm"] == arm and r["report_no"] == n for r in man["rows"]):
        raise SystemExit(f"{arm}-{n} already generated — protocol 37 forbids regeneration")
    print(f"[{utc_now()}] {arm}-{n} start", flush=True)
    row = summarize_a(exph2_ablate.run_report(n, c)) if arm == "A" else summarize_f2(exph2_regen.run_report(n, c))
    man["rows"].append(row)
    write_json(MANIFEST, man)
    print(f"[{utc_now()}] {arm}-{n} done: {json.dumps(row, ensure_ascii=False)}", flush=True)
    return row


def failed_total(man: dict) -> int:
    return sum(r.get("failed_attempts", 0) + r.get("suspected_hidden_failed_attempts", 0) for r in man["rows"])


def cost_total(man: dict) -> float:
    return sum(r.get("cost_usd", 0) for r in man["rows"])


def gate_g2(man: dict) -> bool:
    ok = True
    for r in man["rows"]:
        good = r["status"] == "completed" and r["cost_usd"] <= G2_MAX_COST_PER_REPORT and r["warnings"] == 0
        if r["arm"] == "F2":
            good &= (r.get("llm_target_echo") or {}).get("generation") == GENERATION_CAP
        print(f"G2 {r['arm']}-{r['report_no']}: status={r['status']} warnings={r['warnings']} calls={r['calls']}/{r['requests']} "
              f"cost=${r['cost_usd']:.4f} -> {'PASS' if good else 'FAIL'}")
        ok &= good
    # A bodies byte-equal to the dry-run archive (section calls, first attempt)
    for key in ("dynamics", "spatial", "emergence"):
        live = OUT / "a-1" / "raw" / f"req-{key}-try1.json"
        dry = OUT / "dryrun" / "a-1" / f"req-{key}.json"
        same = live.exists() and dry.exists() and live.read_bytes() == dry.read_bytes()
        print(f"G2 A-1 {key}: live body == dry-run body: {same}")
        ok &= same
    ok &= failed_total(man) <= G3_MAX_FAILED_ATTEMPTS
    print(f"G2 overall: {'PASS' if ok else 'FAIL'} (failed attempts so far {failed_total(man)}, cost ${cost_total(man):.4f})")
    return ok


def main() -> int:
    console_utf8()
    ap = argparse.ArgumentParser()
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--pilot", action="store_true")
    g.add_argument("--battery", action="store_true")
    a = ap.parse_args()
    c = IntelliSimClient()
    man = load_manifest()
    if a.pilot:
        do("A", 1, c, man)
        do("F2", 1, c, man)
        return 0 if gate_g2(man) else 1
    if not all(any(r["arm"] == arm and r["report_no"] == 1 for r in man["rows"]) for arm in ("A", "F2")):
        raise SystemExit("run --pilot first")
    for n in range(2, 11):
        for arm in (("A", "F2") if n % 2 else ("F2", "A")):
            if any(r["arm"] == arm and r["report_no"] == n for r in man["rows"]):
                continue
            do(arm, n, c, man)
            if cost_total(man) > G3_MAX_CUMULATIVE_COST or failed_total(man) > G3_MAX_FAILED_ATTEMPTS:
                print(f"G3 STOP: cost ${cost_total(man):.4f}, failed attempts {failed_total(man)}")
                return 2
    print(f"battery complete: {len(man['rows'])} reports, cost ${cost_total(man):.4f}, failed attempts {failed_total(man)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
