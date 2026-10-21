"""exph_pooled.py — post-hoc pooled statistics over the three scoring passes of the primary series (v7, D32).

Input : results/exp-h-rescoring/rubric.csv   (passes 1 and 2, reports 1-10 = primary series)
        results/exp-h2/rubric3.csv            (pass 3, arm «orig», reports 1-10)
        results/exp-h2/OUTCOMES.json          (cross-check of the pass-3 sums and the sign test)
Output: results/_article/POOLED.md (+ stdout)

What it computes (all descriptive, NOT pre-registered — see KEY-NUMBERS §11):
  * per-pass points / errors / hallucinations and D = points / 60
  * pooled D over passes I-III = Σ points / 180; pooled mean errors per report-scoring = Σ errors / 30
  * one-sided sign test of the pre-registered primary outcome (original > no-data in 10 of 10 runs): p = 0.5 ** 10
  * percentile bootstrap 95 % CI for D (per pass and pooled): resampling unit = the REPORT (n = 10, the same drawn
    reports for all three passes, which keeps the within-report correlation), B = 10 000, numpy default_rng(SEED)
  * κ labels on the Landis & Koch (1977) bands for the three recorded pairs (values from OUTCOMES.json / kappa.md)
Deterministic; safe to re-run. Hard-asserts the sums of record (54 / 46 / 40, 7 / 11 / 8, 1 / 9 / 11).
Usage : python harness/exph_pooled.py
"""
from __future__ import annotations

import csv
import json
import re
import sys
from pathlib import Path

import numpy as np

sys.stdout.reconfigure(encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
RUBRIC12 = ROOT / "results" / "exp-h-rescoring" / "rubric.csv"
RUBRIC3 = ROOT / "results" / "exp-h2" / "rubric3.csv"
OUTCOMES = ROOT / "results" / "exp-h2" / "OUTCOMES.json"
OUT = ROOT / "results" / "_article" / "POOLED.md"

SEED = 20261012
B = 10_000
REPORTS = list(range(1, 11))
MAX_PER_REPORT = 6          # 3 phenomena × 2 points
PASSES = (1, 2, 3)


def load() -> dict[int, dict[int, dict[str, int]]]:
    """pass -> report -> {points, errors, hallucinations}."""
    data: dict[int, dict[int, dict[str, int]]] = {1: {}, 2: {}, 3: {}}
    with RUBRIC12.open(encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            n = int(r["report"])
            if n not in REPORTS:
                continue
            p = int(r["pass"])
            assert int(r["points"]) == int(r["p1"]) + int(r["p2"]) + int(r["p3"]), (p, n)
            data[p][n] = {k: int(r[k]) for k in ("points", "errors", "hallucinations")}
    with RUBRIC3.open(encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            if r["arm"] != "orig":
                continue
            n = int(r["report"])
            assert int(r["points"]) == int(r["p1"]) + int(r["p2"]) + int(r["p3"]), n
            data[3][n] = {k: int(r[k]) for k in ("points", "errors", "hallucinations")}
    for p in PASSES:
        assert sorted(data[p]) == REPORTS, (p, sorted(data[p]))
    return data


def ua_num(s: str) -> str:
    """Decimal comma for a Ukrainian print form."""
    return re.sub(r"(\d)\.(\d)", r"\1,\2", s)


def landis_koch(k: float) -> str:
    for lo, label in ((0.81, "almost perfect"), (0.61, "substantial"), (0.41, "moderate"), (0.21, "fair"), (0.0, "slight")):
        if k >= lo:
            return label
    return "poor"


def main() -> None:
    data = load()
    out = json.loads(OUTCOMES.read_text(encoding="utf-8"))

    pts = {p: np.array([data[p][n]["points"] for n in REPORTS]) for p in PASSES}
    err = {p: np.array([data[p][n]["errors"] for n in REPORTS]) for p in PASSES}
    hal = {p: np.array([data[p][n]["hallucinations"] for n in REPORTS]) for p in PASSES}

    sums = {p: int(pts[p].sum()) for p in PASSES}
    assert (sums[1], sums[2], sums[3]) == (54, 46, 40), sums
    assert tuple(int(err[p].sum()) for p in PASSES) == (7, 11, 8)
    assert tuple(int(hal[p].sum()) for p in PASSES) == (1, 9, 11)
    assert out["arms"]["orig"]["points"] == 40 and out["arms"]["orig"]["errors"] == 8

    pooled_pts = int(sum(sums.values()))
    pooled_err = int(sum(int(err[p].sum()) for p in PASSES))
    pooled_hal = int(sum(int(hal[p].sum()) for p in PASSES))
    n_cells = len(REPORTS) * len(PASSES)            # 30 report-scorings
    max_pts = n_cells * MAX_PER_REPORT               # 180
    D_pooled = pooled_pts / max_pts
    e_pooled = pooled_err / n_cells

    # one-sided sign test of the pre-registered primary outcome (protocol 37 §6): 10 wins of 10
    st = out["primary"]["sign_test"]
    wins = st["positive"]
    assert (wins, st["negative"], st["ties"]) == (10, 0, 0), st
    p_sign = 0.5 ** 10
    assert abs(st["p_one_sided"] - p_sign) < 1e-12, (st["p_one_sided"], p_sign)

    # bootstrap over reports (the same resample for all passes)
    rng = np.random.default_rng(SEED)
    idx = rng.integers(0, len(REPORTS), size=(B, len(REPORTS)))
    tot = pts[1] + pts[2] + pts[3]                   # per-report 3-pass totals (of 18)
    ci = {}
    for p in PASSES:
        dstar = pts[p][idx].sum(axis=1) / (len(REPORTS) * MAX_PER_REPORT)
        ci[p] = tuple(np.percentile(dstar, [2.5, 97.5]))
    dstar_pooled = tot[idx].sum(axis=1) / max_pts
    ci_pooled = tuple(np.percentile(dstar_pooled, [2.5, 97.5]))
    estar = (err[1] + err[2] + err[3])[idx].sum(axis=1) / n_cells
    ci_err = tuple(np.percentile(estar, [2.5, 97.5]))

    kappas = {
        "pass 1 – pass 2 (30 items)": 0.437,
        "pass 2 – pass 3 (30 items)": 0.646,
        "pass 1 – pass 3 (30 items)": 0.244,
        "original – F2, pass 3 (30 items)": 0.305,
    }
    def _k(block):  # OUTCOMES.json blocks carry the unweighted kappa under one of these names
        for name in ("kappa", "kappa_unweighted", "k"):
            if name in block:
                return float(block[name])
        raise KeyError(list(block))
    rec = {"pass 1 – pass 3 (30 items)": _k(out["pass1_vs_pass3_orig"]),
           "pass 2 – pass 3 (30 items)": _k(out["pass2_vs_pass3_orig"]),
           "original – F2, pass 3 (30 items)": _k(out["orig_vs_F2"])}
    for key, val in rec.items():
        assert abs(val - kappas[key]) < 0.005, (key, val, kappas[key])

    lines = [
        "# POOLED — post-hoc, descriptive statistics over the three scoring passes (v7, D32; `harness/exph_pooled.py`)",
        "",
        "> **Not pre-registered.** Protocol 36 defines pass 1 as the scoring of record; protocol 37 §6 defines the primary outcome",
        "> (original > no-data per run, one-sided sign test). Everything below is a post-hoc description over the three scorings",
        "> of the primary series (reports 1–10) and never replaces the pre-registered verdicts (KEY-NUMBERS §2, §8, §10).",
        "",
        "## 1. Per pass (reports 1–10)",
        "",
        "| Pass | Points / 60 | D | Errors (mean / report) | Hallucinated phenomena | D 95 % CI (bootstrap by report) |",
        "|---|---|---|---|---|---|",
    ]
    for p in PASSES:
        lines.append(f"| {p} | {sums[p]} | {sums[p] / 60:.3f} | {int(err[p].sum())} ({err[p].mean():.1f}) | {int(hal[p].sum())} | [{ci[p][0]:.3f}; {ci[p][1]:.3f}] |")
    lines += [
        "",
        "## 2. Pooled over passes I–III",
        "",
        "| Quantity | Value | 95 % CI (bootstrap by report) | Print form (UA) |",
        "|---|---|---|---|",
        f"| D pooled = Σ points / 180 | {pooled_pts}/{max_pts} = {D_pooled:.4f} | [{ci_pooled[0]:.3f}; {ci_pooled[1]:.3f}] | {ua_num(f'«{D_pooled * 100:.1f} % (95 % ДІ {ci_pooled[0]:.2f}–{ci_pooled[1]:.2f})»')} |",
        f"| Mean errors per report-scoring = Σ errors / 30 | {pooled_err}/{n_cells} = {e_pooled:.4f} | [{ci_err[0]:.3f}; {ci_err[1]:.3f}] | {ua_num(f'«{e_pooled:.2f} помилки на звіт»')} |",
        f"| Mean hallucinated phenomena per report-scoring | {pooled_hal}/{n_cells} = {pooled_hal / n_cells:.2f} | — | — |",
        f"| One-sided sign test, 10 wins of 10 | p = 0,5^10 = {p_sign:.10f} = 1/1024 | — | «p = 2^−10 ≈ 0,001» |",
        "",
        "## 3. Bootstrap method",
        "",
        f"- Resampling unit = the report (n = 10): draw 10 reports with replacement, B = {B}, `numpy.random.default_rng({SEED})`,",
        "  `rng.integers(0, 10, size=(B, 10))`; the same drawn reports are used for all three passes, so the pooled statistic keeps",
        "  the within-report correlation across passes. Percentile interval (2,5 / 97,5 %, `numpy.percentile`, linear interpolation).",
        "- D moves in steps of 1/60 per pass, so the bounds are coarse; items inside a report are not independent (3 per report),",
        "  which the report-level resampling respects. Resampling the 30 report-scorings independently would give a narrower,",
        "  anti-conservative interval and is not used.",
        "",
        "## 4. κ labels (Landis & Koch 1977 bands: 0,21–0,40 fair, 0,41–0,60 moderate, 0,61–0,80 substantial)",
        "",
        "| Pair | κ | Label | UA |",
        "|---|---|---|---|",
    ]
    ua = {"fair": "слабка", "moderate": "помірна", "substantial": "суттєва", "almost perfect": "майже повна", "slight": "незначна", "poor": "відсутня"}
    for key, val in kappas.items():
        lab = landis_koch(val)
        lines.append(f"| {key} | {val:.2f} | {lab} | {ua.get(lab, lab)} |")
    lines += ["", f"Sign test of record (OUTCOMES.json primary.sign_test): {wins} positive, 0 negative, 0 ties.", ""]
    text = "\n".join(lines).replace(".replace('.', ',')", "")
    # decimal comma inside the UA print forms only
    text = text.replace("«77.8 %", "«77,8 %").replace("ДІ 0.", "ДІ 0,").replace("–0.", "–0,").replace("«0.87 помилки", "«0,87 помилки")
    OUT.write_text(text, encoding="utf-8")
    print(text)


if __name__ == "__main__":
    main()
