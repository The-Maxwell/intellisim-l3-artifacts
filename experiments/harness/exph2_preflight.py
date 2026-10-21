"""EXP-H2 preflight (gate G0) — read-only GETs against the running stack.

Checks, for the 10 primary EXP-H source simulations:
  * GET /simulations/:id                -> status == finished
  * GET /simulations/:id/results/summary -> step_range.max equals the EXP-H window end
  * GET /simulations/:id/analyses        -> the original EXP-H analysis ids are present
and that GET /llms lists o4-mini (provider "openai") with the results_analysis capability.

No state-changing request is made. Writes results/exp-h2/preflight.json.
Usage: python harness/exph2_preflight.py
"""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")

REPO = Path(__file__).resolve().parents[1]
SIBLING_HARNESS = REPO.parent / "intellisim.experiments" / "harness"
sys.path.insert(0, str(SIBLING_HARNESS))
from intellisim_client import IntelliSimClient  # noqa: E402

EXPH = REPO / "results" / "exp-h"
OUT = REPO / "results" / "exp-h2"

# EXP-H window end per primary report (manifest.json steps_covered.to)
EXPECTED_MAX = {1: 45, 2: 44, 3: 599, 4: 899, 5: 97, 6: 45, 7: 10, 8: 85, 9: 227, 10: 173}


def main() -> int:
    manifest = json.loads((EXPH / "manifest.json").read_text(encoding="utf-8"))
    c = IntelliSimClient()
    rows, ok = [], True
    for n in range(1, 11):
        req = json.loads((EXPH / f"report-{n}" / "request.json").read_text(encoding="utf-8"))
        rep = json.loads((EXPH / f"report-{n}" / "report.json").read_text(encoding="utf-8"))
        a = rep.get("analysis", rep)
        sim_id = a["simulation_id"]
        orig_aid = a["id"]
        sim = c.get_simulation(sim_id)
        summ = c.results_summary(sim_id)
        anl = c.list_analyses(sim_id)
        items = anl.get("items", anl) if isinstance(anl, dict) else anl
        aids = {x.get("id") for x in items} if isinstance(items, list) else set()
        smax = (summ.get("step_range") or {}).get("max")
        row = {
            "report": n, "sim_id": sim_id, "sim_name": sim.get("name"), "status": sim.get("status"),
            "has_results": summ.get("has_results"), "step_range_max": smax,
            "expected_max": EXPECTED_MAX[n], "orig_analysis_present": orig_aid in aids,
            "language": req.get("language"),
        }
        row["pass"] = (row["status"] == "finished" and smax == EXPECTED_MAX[n]
                       and row["orig_analysis_present"])
        ok &= row["pass"]
        rows.append(row)
        print(f"report {n:2d}  {sim_id}  {row['sim_name']!s:24s} status={row['status']} "
              f"max={smax} (exp {EXPECTED_MAX[n]}) orig_analysis={row['orig_analysis_present']} "
              f"-> {'PASS' if row['pass'] else 'FAIL'}")
    llms = c.get_llms()
    found, max_tokens_param = False, None
    providers = llms.get("list", []) if isinstance(llms, dict) else llms
    for prov in providers:
        for m in prov.get("models", []):
            if prov.get("uid") == "openai" and m.get("uid") == "o4-mini":
                found = "results_analysis" in (m.get("capabilities") or []) and bool(m.get("reasoning"))
                max_tokens_param = next((p for p in m.get("config_params", []) if p.get("key") == "max_tokens"), None)
    print(f"o4-mini with results_analysis + reasoning visible: {found}; max_tokens param: {max_tokens_param}")
    found = found and max_tokens_param is not None and max_tokens_param.get("type") == "uint"
    ok &= found
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "preflight.json").write_text(json.dumps({
        "checked_at_utc": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "rows": rows, "o4mini_results_analysis": found, "gate_G0": "PASS" if ok else "FAIL",
    }, ensure_ascii=False, indent=2), encoding="utf-8")
    print("G0:", "PASS" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
