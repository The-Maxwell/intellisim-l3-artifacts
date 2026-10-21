"""exph_rescore.py — EXP-H scoring sheets → machine-readable rubric.csv, with REPORT.md cross-check.

Pass 1 (original, 2026-07-24):     results/exp-h/report-N/scoring.md               (N = 1..12)
Pass 2 (blind re-scoring, Task 3): results/exp-h-rescoring/scoring-2/report-N.md   (same sheet format; optional)

Output: results/exp-h-rescoring/rubric.csv — one row per (pass, report).
Columns: pass, report, set, scenario, family, lang, model, sim_id, analysis_id,
         p1, p2, p3, points, errors, hallucinations, useful, scorer, scored

Checks (pass 1 only, hard assertions):
  * per-report P1/P2/P3, sum, errors, hallucinations, useful == the "Report set" table in results/exp-h/REPORT.md
  * primary totals: 54/60 points, 7 errors, 1 hallucination, 10/10 useful
  * en 29/30 & 1 error; uk 25/30 & 6 errors; o4-mini pair(1,2) 11/12 & 1 error; o3 pair(11,12) 12/12 & 0 errors

Usage:  python harness/exph_rescore.py            # pass 1 (+ pass 2 rows if scoring-2/ exists)
        python harness/exph_rescore.py --pass2    # require pass 2 to be complete (12 files) and print both passes
"""
from __future__ import annotations

import csv
import json
import re
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
EXPH = ROOT / "results" / "exp-h"
RESC = ROOT / "results" / "exp-h-rescoring"
OUT = RESC / "rubric.csv"

FAMILY = {  # scenario family from the manifest's source_run prefix
    "EXP-C": "evacuation-room",
    "EXP-B": "evacuation-rimea",
    "EXP-E": "evacuation-llm-decision",
    "EXP-F": "wildfire",
    "EXP-G": "epidemic",
}

RE_META = re.compile(
    r"Meta:\s*sim_id=(?P<sim>\w+)\s+analysis_id=(?P<aid>\w+)\s+lang=(?P<lang>\w+)\s+"
    r"llm_target=(?P<model>[\w.-]+)\s+generated=(?P<gen>\S+)\s+scorer=(?P<scorer>.+?)\s+scored=(?P<scored>\S+)"
)
RE_ROW = re.compile(r"^\|\s*(?P<n>[123])\s*\|\s*(?P<s>[012])\s*\|", re.M)
RE_POINTS = re.compile(r"\*\*Phenomenon points:\s*(?P<p>\d+)/6\*\*")
RE_ERR = re.compile(r"Factual errors.*?count\s*=\s*(?P<c>\d+)", re.S)
RE_HAL = re.compile(r"Hallucinated phenomena.*?count\s*=\s*(?P<c>\d+)", re.S)
RE_USE = re.compile(r"Actionable recommendations:\s*(?P<u>[yn])\b")


def parse_sheet(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    m = RE_META.search(text)
    if not m:
        raise SystemExit(f"{path}: Meta line not found")
    scores = {int(r.group("n")): int(r.group("s")) for r in RE_ROW.finditer(text)}
    if sorted(scores) != [1, 2, 3]:
        raise SystemExit(f"{path}: expected rows 1..3 with 0/1/2 scores, got {scores}")
    pts = RE_POINTS.search(text)
    err = RE_ERR.search(text)
    hal = RE_HAL.search(text)
    use = RE_USE.search(text)
    for name, hit in (("points", pts), ("errors", err), ("hallucinations", hal), ("useful", use)):
        if not hit:
            raise SystemExit(f"{path}: '{name}' field not found")
    points = int(pts.group("p"))
    if points != sum(scores.values()):
        raise SystemExit(f"{path}: declared points {points} != sum of rows {sum(scores.values())}")
    return {
        "sim_id": m.group("sim"),
        "analysis_id": m.group("aid"),
        "lang": m.group("lang"),
        "model": m.group("model"),
        "scorer": m.group("scorer").strip(),
        "scored": m.group("scored"),
        "p1": scores[1],
        "p2": scores[2],
        "p3": scores[3],
        "points": points,
        "errors": int(err.group("c")),
        "hallucinations": int(hal.group("c")),
        "useful": use.group("u"),
    }


def load_manifest() -> dict[int, dict]:
    man = json.loads((EXPH / "manifest.json").read_text(encoding="utf-8"))
    return {r["report"]: r for r in man["reports"]}


def parse_report_md() -> dict[int, dict]:
    """Rows of the 'Report set' table in REPORT.md -> {report: {...}}."""
    text = (EXPH / "REPORT.md").read_text(encoding="utf-8")
    rows = {}
    pat = re.compile(
        r"^\|\s*(\d+)\s*\|[^|]*\|\s*(en|uk)\s*\|\s*(o4-mini|o3)\s*\|\s*(\d)\s*/\s*(\d)\s*/\s*(\d)\s*\|"
        r"\s*\*\*(\d)\*\*\s*\|\s*(\d)\s*\|\s*(\d)\s*\|\s*(y|n)\s*\|",
        re.M,
    )
    for m in pat.finditer(text):
        n = int(m.group(1))
        rows[n] = {
            "lang": m.group(2), "model": m.group(3),
            "p1": int(m.group(4)), "p2": int(m.group(5)), "p3": int(m.group(6)),
            "points": int(m.group(7)), "errors": int(m.group(8)),
            "hallucinations": int(m.group(9)), "useful": m.group(10),
        }
    if sorted(rows) != list(range(1, 13)):
        raise SystemExit(f"REPORT.md: expected 12 table rows, parsed {sorted(rows)}")
    return rows


def collect(pass_no: int, manifest: dict[int, dict]) -> list[dict]:
    out = []
    for n in range(1, 13):
        if pass_no == 1:
            path = EXPH / f"report-{n}" / "scoring.md"
        else:
            path = RESC / "scoring-2" / f"report-{n}.md"
        if not path.exists():
            if pass_no == 1:
                raise SystemExit(f"missing {path}")
            continue
        d = parse_sheet(path)
        mr = manifest[n]
        if d["sim_id"] != mr["sim_id"] or d["analysis_id"] != mr["analysis_id"]:
            raise SystemExit(f"report {n} pass {pass_no}: sim/analysis id differs from manifest")
        if d["lang"] != mr["language"] or d["model"] != mr["llm_target"]:
            raise SystemExit(f"report {n} pass {pass_no}: lang/model differs from manifest")
        src = mr["source_run"]
        d.update({
            "pass": pass_no,
            "report": n,
            "set": "primary" if n <= 10 else "stretch",
            "scenario": src,
            "family": FAMILY[src.split()[0]],
        })
        out.append(d)
    return out


def summarise(rows: list[dict]) -> dict:
    prim = [r for r in rows if r["set"] == "primary"]

    def s(rs, k):
        return sum(r[k] for r in rs)

    return {
        "primary_points": s(prim, "points"), "primary_errors": s(prim, "errors"),
        "primary_hallucinations": s(prim, "hallucinations"),
        "primary_useful": sum(r["useful"] == "y" for r in prim),
        "en_points": s([r for r in prim if r["lang"] == "en"], "points"),
        "en_errors": s([r for r in prim if r["lang"] == "en"], "errors"),
        "uk_points": s([r for r in prim if r["lang"] == "uk"], "points"),
        "uk_errors": s([r for r in prim if r["lang"] == "uk"], "errors"),
        "o4mini_pair_points": s([r for r in rows if r["report"] in (1, 2)], "points"),
        "o4mini_pair_errors": s([r for r in rows if r["report"] in (1, 2)], "errors"),
        "o3_pair_points": s([r for r in rows if r["report"] in (11, 12)], "points"),
        "o3_pair_errors": s([r for r in rows if r["report"] in (11, 12)], "errors"),
        "perfect_6": sum(r["points"] == 6 for r in prim),
        "ge_5": sum(r["points"] >= 5 for r in prim),
    }


# From results/exp-h/REPORT.md (verdict line, en/uk table, o4-mini/o3 table).
# CORRECTION (2026-09-08): REPORT.md prose says "7 of 10 primary reports scored a perfect 6/6"; the table
# (and the 12 scoring sheets) give SIX perfect reports (1, 3, 7, 8, 9, 10). Sum 54, mean 5.4 and "nine of ten
# >= 5/6" are correct. The article must use 6/10, see results/_article/KEY-NUMBERS.md.
EXPECTED_PASS1 = {
    "primary_points": 54, "primary_errors": 7, "primary_hallucinations": 1, "primary_useful": 10,
    "en_points": 29, "en_errors": 1, "uk_points": 25, "uk_errors": 6,
    "o4mini_pair_points": 11, "o4mini_pair_errors": 1, "o3_pair_points": 12, "o3_pair_errors": 0,
    "perfect_6": 6, "ge_5": 9,
}

FIELDS = ["pass", "report", "set", "scenario", "family", "lang", "model", "sim_id", "analysis_id",
          "p1", "p2", "p3", "points", "errors", "hallucinations", "useful", "scorer", "scored"]


def main(argv: list[str]) -> int:
    require_pass2 = "--pass2" in argv
    manifest = load_manifest()
    rows1 = collect(1, manifest)

    # --- cross-check pass 1 against REPORT.md, row by row and in aggregate
    table = parse_report_md()
    for r in rows1:
        t = table[r["report"]]
        for k in ("lang", "model", "p1", "p2", "p3", "points", "errors", "hallucinations", "useful"):
            if r[k] != t[k]:
                raise SystemExit(f"report {r['report']}: scoring.md {k}={r[k]} but REPORT.md says {t[k]}")
    summ1 = summarise(rows1)
    bad = {k: (summ1[k], v) for k, v in EXPECTED_PASS1.items() if summ1[k] != v}
    if bad:
        raise SystemExit(f"pass-1 aggregates differ from REPORT.md: {bad}")

    rows2 = collect(2, manifest)
    if require_pass2 and len(rows2) != 12:
        raise SystemExit(f"--pass2: expected 12 pass-2 sheets in {RESC / 'scoring-2'}, found {len(rows2)}")

    RESC.mkdir(parents=True, exist_ok=True)
    with OUT.open("w", newline="", encoding="utf-8") as fh:
        w = csv.DictWriter(fh, fieldnames=FIELDS)
        w.writeheader()
        for r in rows1 + rows2:
            w.writerow({k: r[k] for k in FIELDS})

    print(f"wrote {OUT.relative_to(ROOT)}: {len(rows1)} pass-1 rows, {len(rows2)} pass-2 rows")
    print("pass 1 (== REPORT.md):", json.dumps(summ1))
    if rows2:
        print(f"pass 2 ({len(rows2)} reports):", json.dumps(summarise(rows2)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
