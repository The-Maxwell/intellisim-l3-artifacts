"""kappa.py — inter-pass agreement for the EXP-H rubric (pass 1 original vs pass 2 blind).

Input : results/exp-h-rescoring/rubric.csv (needs pass 1 and pass 2 rows for all 12 reports;
        produce with `python harness/exph_rescore.py --pass2`)
Output: results/exp-h-rescoring/kappa.md  (+ stdout summary)

Statistics (no external deps):
  * Cohen's kappa, unweighted and linear-weighted, over the 36 phenomenon items (12 x 3) and over the 30 primary items
  * % exact agreement, confusion matrix (pass1 rows x pass2 cols, categories 0/1/2)
  * factual-error and hallucination counts: exact-match rate, mean |diff|, totals per pass
  * pass-2 aggregates in the pass-1 layout + pre-registered criteria under pass 2 alone
  * disagreement log (items where scores differ, with both evidence quotes read from the sheets)
Usage : python harness/kappa.py
"""
from __future__ import annotations

import csv
import re
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
RESC = ROOT / "results" / "exp-h-rescoring"
RUBRIC = RESC / "rubric.csv"
OUT = RESC / "kappa.md"
EXPH = ROOT / "results" / "exp-h"

CATS = (0, 1, 2)


def cohen_kappa(a: list[int], b: list[int], weighted: bool) -> float:
    n = len(a)
    k = len(CATS)
    obs = [[0.0] * k for _ in range(k)]
    for x, y in zip(a, b):
        obs[x][y] += 1
    row = [sum(obs[i]) for i in range(k)]
    col = [sum(obs[i][j] for i in range(k)) for j in range(k)]
    exp = [[row[i] * col[j] / n for j in range(k)] for i in range(k)]
    if weighted:
        w = [[abs(i - j) / (k - 1) for j in range(k)] for i in range(k)]
    else:
        w = [[0.0 if i == j else 1.0 for j in range(k)] for i in range(k)]
    dis_o = sum(w[i][j] * obs[i][j] for i in range(k) for j in range(k)) / n
    dis_e = sum(w[i][j] * exp[i][j] for i in range(k) for j in range(k)) / n
    if dis_e == 0:
        return 1.0
    return 1.0 - dis_o / dis_e


def confusion(a: list[int], b: list[int]) -> list[list[int]]:
    m = [[0] * 3 for _ in range(3)]
    for x, y in zip(a, b):
        m[x][y] += 1
    return m


def interpret(k: float) -> str:  # Landis & Koch 1977 bands
    for lo, label in ((0.81, "almost perfect"), (0.61, "substantial"), (0.41, "moderate"), (0.21, "fair"), (0.0, "slight")):
        if k >= lo:
            return label
    return "poor"


def load() -> tuple[dict[int, dict], dict[int, dict]]:
    p1, p2 = {}, {}
    with RUBRIC.open(encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            n = int(r["report"])
            (p1 if r["pass"] == "1" else p2)[n] = r
    if sorted(p1) != list(range(1, 13)) or sorted(p2) != list(range(1, 13)):
        raise SystemExit(f"rubric.csv must hold 12 pass-1 and 12 pass-2 rows (have {len(p1)} / {len(p2)}); run exph_rescore.py --pass2")
    return p1, p2


def quotes(path: Path) -> dict[int, str]:
    """Row -> evidence quote from a scoring sheet."""
    out = {}
    for m in re.finditer(r"^\|\s*([123])\s*\|\s*[012]\s*\|\s*(.*?)\s*\|\s*$", path.read_text(encoding="utf-8"), re.M):
        out[int(m.group(1))] = m.group(2)
    return out


def summarise(rows: dict[int, dict]) -> dict:
    prim = [rows[n] for n in range(1, 11)]
    g = lambda rs, k: sum(int(r[k]) for r in rs)  # noqa: E731
    return {
        "points": g(prim, "points"), "errors": g(prim, "errors"), "halluc": g(prim, "hallucinations"),
        "useful": sum(r["useful"] == "y" for r in prim),
        "en_points": g([r for r in prim if r["lang"] == "en"], "points"), "en_errors": g([r for r in prim if r["lang"] == "en"], "errors"),
        "uk_points": g([r for r in prim if r["lang"] == "uk"], "points"), "uk_errors": g([r for r in prim if r["lang"] == "uk"], "errors"),
        "o4_pair": g([rows[1], rows[2]], "points"), "o4_pair_err": g([rows[1], rows[2]], "errors"),
        "o3_pair": g([rows[11], rows[12]], "points"), "o3_pair_err": g([rows[11], rows[12]], "errors"),
        "perfect": sum(int(r["points"]) == 6 for r in prim), "ge5": sum(int(r["points"]) >= 5 for r in prim),
    }


def main() -> int:
    p1, p2 = load()
    items1, items2, labels = [], [], []
    for n in range(1, 13):
        for k in ("p1", "p2", "p3"):
            items1.append(int(p1[n][k]))
            items2.append(int(p2[n][k]))
            labels.append((n, int(k[1])))
    prim_idx = [i for i, (n, _) in enumerate(labels) if n <= 10]

    def stats(idx):
        a = [items1[i] for i in idx]
        b = [items2[i] for i in idx]
        return {
            "n": len(a), "agree": sum(x == y for x, y in zip(a, b)) / len(a),
            "kappa": cohen_kappa(a, b, False), "wkappa": cohen_kappa(a, b, True), "conf": confusion(a, b),
            "sum1": sum(a), "sum2": sum(b),
        }

    all36 = stats(range(36))
    prim30 = stats(prim_idx)

    err1 = [int(p1[n]["errors"]) for n in range(1, 13)]
    err2 = [int(p2[n]["errors"]) for n in range(1, 13)]
    hal1 = [int(p1[n]["hallucinations"]) for n in range(1, 13)]
    hal2 = [int(p2[n]["hallucinations"]) for n in range(1, 13)]
    use1 = [p1[n]["useful"] for n in range(1, 13)]
    use2 = [p2[n]["useful"] for n in range(1, 13)]

    s1, s2 = summarise(p1), summarise(p2)
    crit1_p2 = s2["points"] >= 42
    crit2_p2 = s2["errors"] <= 10

    md = ["# Inter-pass agreement — EXP-H rubric, pass 1 (original, 2026-07-24) vs pass 2 (blind, 2026-09-08)\n",
          f"> Generated by `harness/kappa.py` from `rubric.csv`. Protocol: `PROTOCOL.md`. Both passes are LLM-assisted (D12); κ measures reproducibility of the rubric's application. Original scores unchanged.\n",
          "## Phenomenon items (0/1/2)\n",
          "| Item set | n | Exact agreement | Cohen's κ | Linear-weighted κ | Σ pass 1 | Σ pass 2 |", "|---|---|---|---|---|---|---|",
          f"| all 12 reports | {all36['n']} | {all36['agree']:.3f} | {all36['kappa']:.3f} ({interpret(all36['kappa'])}) | {all36['wkappa']:.3f} | {all36['sum1']} | {all36['sum2']} |",
          f"| primary 1–10 | {prim30['n']} | {prim30['agree']:.3f} | {prim30['kappa']:.3f} ({interpret(prim30['kappa'])}) | {prim30['wkappa']:.3f} | {prim30['sum1']} / 60 | {prim30['sum2']} / 60 |\n",
          "Confusion matrix, all 36 items (rows = pass 1 score, columns = pass 2 score 0 / 1 / 2):\n",
          "| pass 1 \\ pass 2 | 0 | 1 | 2 |", "|---|---|---|---|"]
    for i in CATS:
        md.append(f"| **{i}** | " + " | ".join(str(all36["conf"][i][j]) for j in CATS) + " |")
    md += ["\n## Per-report counts\n",
           "| # | Lang | Model | Points p1 | Points p2 | Errors p1 | Errors p2 | Halluc p1 | Halluc p2 | Useful p1 | Useful p2 |",
           "|---|---|---|---|---|---|---|---|---|---|---|"]
    for n in range(1, 13):
        md.append(f"| {n} | {p1[n]['lang']} | {p1[n]['model']} | {p1[n]['points']} | {p2[n]['points']} | {err1[n-1]} | {err2[n-1]} | {hal1[n-1]} | {hal2[n-1]} | {use1[n-1]} | {use2[n-1]} |")
    md.append(f"| **Σ 1–10** | | | **{s1['points']}** | **{s2['points']}** | **{s1['errors']}** | **{s2['errors']}** | **{s1['halluc']}** | **{s2['halluc']}** | {s1['useful']}/10 | {s2['useful']}/10 |")
    md.append(f"| Σ 11–12 | | | {s1['o3_pair']} | {s2['o3_pair']} | {s1['o3_pair_err']} | {s2['o3_pair_err']} | | | | |\n")
    mad_e = sum(abs(x - y) for x, y in zip(err1, err2)) / 12
    mad_h = sum(abs(x - y) for x, y in zip(hal1, hal2)) / 12
    md += ["| Count | Exact match (12 reports) | Mean abs. diff. | Total p1 | Total p2 |", "|---|---|---|---|---|",
           f"| Factual errors | {sum(x == y for x, y in zip(err1, err2))}/12 | {mad_e:.2f} | {sum(err1)} | {sum(err2)} |",
           f"| Hallucinated phenomena | {sum(x == y for x, y in zip(hal1, hal2))}/12 | {mad_h:.2f} | {sum(hal1)} | {sum(hal2)} |",
           f"| Usefulness (y/n) | {sum(x == y for x, y in zip(use1, use2))}/12 | — | {use1.count('y')} y | {use2.count('y')} y |\n",
           "## Pass-2 aggregates in the pass-1 layout (primary reports 1–10)\n",
           "| Quantity | Pass 1 | Pass 2 |", "|---|---|---|",
           f"| Phenomenon points | {s1['points']}/60 ({s1['points']/60:.1%}) | {s2['points']}/60 ({s2['points']/60:.1%}) |",
           f"| Perfect reports (6/6) | {s1['perfect']} | {s2['perfect']} |",
           f"| Reports ≥ 5/6 | {s1['ge5']} | {s2['ge5']} |",
           f"| Factual errors (mean / report) | {s1['errors']} ({s1['errors']/10:.1f}) | {s2['errors']} ({s2['errors']/10:.1f}) |",
           f"| Hallucinated phenomena | {s1['halluc']} | {s2['halluc']} |",
           f"| Usefulness | {s1['useful']}/10 | {s2['useful']}/10 |",
           f"| en points / errors | {s1['en_points']}/30 / {s1['en_errors']} | {s2['en_points']}/30 / {s2['en_errors']} |",
           f"| uk points / errors | {s1['uk_points']}/30 / {s1['uk_errors']} | {s2['uk_points']}/30 / {s2['uk_errors']} |",
           f"| o4-mini pair (1, 2) points / errors | {s1['o4_pair']}/12 / {s1['o4_pair_err']} | {s2['o4_pair']}/12 / {s2['o4_pair_err']} |",
           f"| o3 pair (11, 12) points / errors | {s1['o3_pair']}/12 / {s1['o3_pair_err']} | {s2['o3_pair']}/12 / {s2['o3_pair_err']} |",
           f"| Criterion 1 (≥ 42 points) | PASS | {'PASS' if crit1_p2 else 'FAIL'} |",
           f"| Criterion 2 (≤ 10 errors) | PASS | {'PASS' if crit2_p2 else 'FAIL'} |\n",
           "## Disagreement log (phenomenon items where the passes differ)\n"]
    dis = [(n, k) for i, (n, k) in enumerate(labels) if items1[i] != items2[i]]
    if not dis:
        md.append("None — all 36 items identical.\n")
    else:
        md += ["| # | Phenomenon | Pass 1 | Pass 2 | Pass-1 evidence | Pass-2 evidence |", "|---|---|---|---|---|---|"]
        for n, k in dis:
            q1 = quotes(EXPH / f"report-{n}" / "scoring.md").get(k, "")
            q2 = quotes(RESC / "scoring-2" / f"report-{n}.md").get(k, "")
            md.append(f"| {n} | {k} | {p1[n]['p%d' % k]} | {p2[n]['p%d' % k]} | {q1} | {q2} |")
        md.append("")
    md.append("Error / hallucination transcripts of both passes: `results/exp-h/report-N/scoring.md` (pass 1) and `scoring-2/report-N.md` (pass 2). Nothing was adjudicated; the article reports both passes and κ.\n")
    OUT.write_text("\n".join(md), encoding="utf-8")
    print(f"wrote {OUT.relative_to(ROOT)}")
    print(f"36 items: agree {all36['agree']:.3f}, kappa {all36['kappa']:.3f}, weighted {all36['wkappa']:.3f}; primary 30: agree {prim30['agree']:.3f}, kappa {prim30['kappa']:.3f}, weighted {prim30['wkappa']:.3f}")
    print(f"pass 2 primary: {s2['points']}/60, errors {s2['errors']}, halluc {s2['halluc']}, useful {s2['useful']}/10; criteria {'PASS' if crit1_p2 else 'FAIL'}/{'PASS' if crit2_p2 else 'FAIL'}")
    print(f"disagreements: {len(dis)} items {dis}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
