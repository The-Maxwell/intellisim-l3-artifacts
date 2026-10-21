"""EXP-H2 pass 3 — parse the 30 blinded scoring sheets and compute the pre-registered outcomes (protocol 37 §6).

Input : results/exp-h2/scoring-3/Rxx.md (30, pass-1 sheet layout), scoring-3/_mapping.json,
        results/exp-h-rescoring/rubric.csv (passes 1-2), results/exp-h2/manifest.json
Output: results/exp-h2/rubric3.csv, results/exp-h2/OUTCOMES.json (+ stdout)
Usage : python harness/exph2_score.py
"""

from __future__ import annotations

import csv
import json
import math
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from exph2_common import OUT, PRIMARY, REPO, console_utf8, write_json  # noqa: E402
from exph_rescore import parse_sheet  # noqa: E402
from kappa import cohen_kappa  # noqa: E402

S3 = OUT / "scoring-3"
ARMS = ("orig", "F2", "A")
# protocol 37 §4 — fixed before generation
CLASS_M = ["1P2", "2P2", "3P1", "4P2", "5P3", "6P3", "7P3", "8P2", "8P3", "9P3", "10P3"]
CLASS_P = ["1P1", "2P1", "3P2", "5P1", "7P1", "7P2", "8P1", "9P1", "10P1"]
CLASS_D = ["1P3", "2P3", "3P3", "4P1", "4P3", "5P2", "6P1", "6P2", "9P2", "10P2"]
assert len(CLASS_M) == 11 and len(CLASS_P) == 9 and len(CLASS_D) == 10
assert len(set(CLASS_M + CLASS_P + CLASS_D)) == 30


def item_class(n: int, j: int) -> str:
    key = f"{n}P{j}"
    return "M" if key in CLASS_M else "P" if key in CLASS_P else "D"


def sign_test_one_sided(diffs: list[int]) -> dict:
    pos = sum(d > 0 for d in diffs)
    neg = sum(d < 0 for d in diffs)
    n = pos + neg
    p = sum(math.comb(n, k) for k in range(pos, n + 1)) / 2 ** n if n else 1.0
    return {"positive": pos, "negative": neg, "ties": len(diffs) - n, "p_one_sided": p}


def load_pass(pass_no: int) -> dict[int, dict]:
    rows = {}
    with (REPO / "results" / "exp-h-rescoring" / "rubric.csv").open(encoding="utf-8") as fh:
        for r in csv.DictReader(fh):
            if int(r["pass"]) == pass_no and int(r["report"]) in PRIMARY:
                rows[int(r["report"])] = {k: int(r[k]) for k in ("p1", "p2", "p3", "points", "errors", "hallucinations")}
    return rows


def agg(rows: dict[int, dict]) -> dict:
    pts = sum(r["points"] for r in rows.values())
    return {"points": pts, "of": 60, "D": round(pts / 60, 4), "errors": sum(r["errors"] for r in rows.values()),
            "errors_per_report": round(sum(r["errors"] for r in rows.values()) / 10, 2),
            "hallucinations": sum(r["hallucinations"] for r in rows.values()),
            "useful": sum(r["useful"] == "y" for r in rows.values()) if all("useful" in r for r in rows.values()) else None,
            "perfect": sum(r["points"] == 6 for r in rows.values()),
            "criterion1_pass": pts >= 42, "criterion2_pass": sum(r["errors"] for r in rows.values()) <= 10}


def items(rows: dict[int, dict]) -> list[int]:
    return [rows[n][f"p{j}"] for n in PRIMARY for j in (1, 2, 3)]


def main() -> int:
    console_utf8()
    mapping = json.loads((S3 / "_mapping.json").read_text(encoding="utf-8"))
    by_arm: dict[str, dict[int, dict]] = {a: {} for a in ARMS}
    csv_rows = []
    for m in mapping["reports"]:
        r = parse_sheet(S3 / f"{m['id']}.md")
        if r["sim_id"] != m["id"]:
            raise SystemExit(f"{m['id']}: Meta line altered ({r['sim_id']})")
        by_arm[m["arm"]][m["report_no"]] = r
        csv_rows.append({"id": m["id"], "report": m["report_no"], "arm": m["arm"], "agent": m["agent"],
                         **{k: r[k] for k in ("p1", "p2", "p3", "points", "errors", "hallucinations", "useful")}})
    for a in ARMS:
        if sorted(by_arm[a]) != PRIMARY:
            raise SystemExit(f"arm {a}: missing reports {sorted(set(PRIMARY) - set(by_arm[a]))}")
    csv_rows.sort(key=lambda x: (ARMS.index(x["arm"]), x["report"]))
    with (OUT / "rubric3.csv").open("w", newline="", encoding="utf-8") as fh:
        w = csv.DictWriter(fh, fieldnames=list(csv_rows[0]))
        w.writeheader()
        w.writerows(csv_rows)

    out: dict = {"arms": {a: agg(by_arm[a]) for a in ARMS}}
    # A by item class
    cls = {"M": [0, 0, 0], "P": [0, 0, 0], "D": [0, 0, 0]}   # points, max, n_two
    zeros_d = 0
    for n in PRIMARY:
        for j in (1, 2, 3):
            c, s = item_class(n, j), by_arm["A"][n][f"p{j}"]
            cls[c][0] += s
            cls[c][1] += 2
            cls[c][2] += s == 2
            zeros_d += c == "D" and s == 0
    out["A_by_class"] = {c: {"points": v[0], "max": v[1], "two_point_items": v[2]} for c, v in cls.items()}
    out["A_two_point_items"] = sum(v[2] for v in cls.values())
    out["A_D_items_zero_share"] = round(zeros_d / len(CLASS_D), 3)
    for arm in ("orig", "F2"):
        sub = {c: sum(by_arm[arm][n][f"p{j}"] for n in PRIMARY for j in (1, 2, 3) if item_class(n, j) == c) for c in "MPD"}
        out[f"{arm}_by_class_points"] = sub
    # primary outcome
    diffs = [by_arm["orig"][n]["points"] - by_arm["A"][n]["points"] for n in PRIMARY]
    out["primary"] = {"per_run_orig_minus_A": dict(zip(PRIMARY, diffs)), "sum": sum(diffs),
                      "sign_test": sign_test_one_sided(diffs)}
    out["per_run"] = {n: {a: by_arm[a][n]["points"] for a in ARMS} for n in PRIMARY}
    # original vs F2 (pass 3)
    io, i2 = items(by_arm["orig"]), items(by_arm["F2"])
    out["orig_vs_F2"] = {"item_exact_agreement": round(sum(x == y for x, y in zip(io, i2)) / 30, 3),
                         "kappa": round(cohen_kappa(io, i2, False), 3), "kappa_linear": round(cohen_kappa(io, i2, True), 3),
                         "per_run_points_diff": {n: by_arm["F2"][n]["points"] - by_arm["orig"][n]["points"] for n in PRIMARY}}
    # agreement of pass 3 with passes 1 and 2 on the original reports
    for pn in (1, 2):
        prev = load_pass(pn)
        ip = items(prev)
        out[f"pass{pn}_vs_pass3_orig"] = {
            "item_exact_agreement": round(sum(x == y for x, y in zip(ip, io)) / 30, 3),
            "kappa": round(cohen_kappa(ip, io, False), 3), "kappa_linear": round(cohen_kappa(ip, io, True), 3),
            "pass_points": sum(r["points"] for r in prev.values()), "pass_errors": sum(r["errors"] for r in prev.values()),
            "pass_hallucinations": sum(r["hallucinations"] for r in prev.values())}
    # run metadata
    man = json.loads((OUT / "manifest.json").read_text(encoding="utf-8"))
    for arm in ("A", "F2"):
        rs = [r for r in man["rows"] if r["arm"] == arm]
        wc = [r["wall_clock_s"] for r in rs]
        out[f"{arm}_runs"] = {"n": len(rs), "wall_clock_s_min": min(wc), "wall_clock_s_max": max(wc),
                              "wall_clock_s_mean": round(sum(wc) / len(wc), 1), "cost_usd": round(sum(r["cost_usd"] for r in rs), 4),
                              "warnings": sum(r["warnings"] for r in rs),
                              "failed_attempts": sum(r.get("failed_attempts", 0) + r.get("suspected_hidden_failed_attempts", 0) for r in rs),
                              "models": sorted({m for r in rs for m in r["resolved_models"]})}
    write_json(OUT / "OUTCOMES.json", out)
    print(json.dumps(out, ensure_ascii=False, indent=1))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
