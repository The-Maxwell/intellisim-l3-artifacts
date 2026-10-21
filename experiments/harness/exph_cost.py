"""exph_cost.py — ONE cost definition for EXP-H (Level-3 analysis, 12 reports of 2026-07-24).

Definition of record (see results/_billing/exph-cost.md and DECISIONS D13):
    EXP-H cost = OpenAI usage-export rows dated 2026-07-24 for the models o4-mini and o3
                 (all reasoning-model traffic of that day was EXP-H — reconciliation.md)
                 x published list prices per 1M tokens (verified on PRICE_VERIFIED date),
                 input priced at the uncached rate (cached-input discount ignored: conservative, < $0.01).

The engine's own token accounting (manifest.json / cost.json) is reported alongside ONLY for the
per-report distribution (billing is a daily aggregate and cannot be split per report) and for the
o4-mini output under-count finding. It is NOT the cost of record.

Inputs : results/_billing/completions_usage_*.csv, results/exp-h/manifest.json, results/exp-h/cost.json
Output : results/_billing/exph-cost.md  (+ stdout summary)
Usage  : python harness/exph_cost.py
"""
from __future__ import annotations

import csv
import glob
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
BILL = ROOT / "results" / "_billing"
EXPH = ROOT / "results" / "exp-h"
OUT = BILL / "exph-cost.md"

EXPH_DATE = "2026-07-24"
# USD per 1M tokens (input, output). Re-verified against https://developers.openai.com/api/docs/pricing
# ("Standard" table rows: "o4-mini | $1.10 | $0.275 | - | $4.40", "o3 | $2.00 | $0.50 | - | $8.00").
PRICE = {"o4-mini": (1.10, 4.40), "o3": (2.00, 8.00)}
PRICE_VERIFIED = "2026-09-08"
PRICE_URL = "https://developers.openai.com/api/docs/pricing"

# Expected billing rows (from results/_billing/reconciliation.md, "2026-07-24 activity" table) — hard-asserted.
EXPECTED_BILLING = {"o4-mini": (41, 132_194, 160_920), "o3": (8, 22_001, 16_176)}


def model_key(billing_model: str) -> str | None:
    for k in PRICE:
        if billing_model.startswith(k + "-") or billing_model == k:
            return k
    return None


def billing_rows() -> dict[str, dict]:
    agg = {k: {"requests": 0, "input": 0, "output": 0, "cached": 0, "tiers": set(), "model_ids": set()} for k in PRICE}
    for f in sorted(glob.glob(str(BILL / "completions_usage_*.csv"))):
        with open(f, encoding="utf-8", newline="") as fh:
            for r in csv.DictReader(fh):
                if not r["start_time_iso"].startswith(EXPH_DATE):
                    continue
                k = model_key(r["model"])
                if k is None:
                    continue
                a = agg[k]
                a["requests"] += int(float(r["num_model_requests"]))
                a["input"] += int(float(r["input_tokens"]))
                a["output"] += int(float(r["output_tokens"]))
                a["cached"] += int(float(r["input_cached_tokens"]))
                a["tiers"].add(r["service_tier"])
                a["model_ids"].add(r["model"])
    for k, (req, inp, out) in EXPECTED_BILLING.items():
        got = (agg[k]["requests"], agg[k]["input"], agg[k]["output"])
        if got != (req, inp, out):
            raise SystemExit(f"billing {k} on {EXPH_DATE}: got {got}, reconciliation.md says {(req, inp, out)}")
    return agg


def engine_rows() -> list[dict]:
    man = json.loads((EXPH / "manifest.json").read_text(encoding="utf-8"))
    cost = json.loads((EXPH / "cost.json").read_text(encoding="utf-8"))
    cost_by_report = {row[0]: row for row in cost["rows"]}
    out = []
    for r in man["reports"]:
        t = r["token_usage"]["total"]
        c = cost_by_report[r["report"]]
        if (c[3], c[4], c[5]) != (t["prompt_tokens"], t["completion_tokens"], t["reasoning_tokens"]):
            raise SystemExit(f"report {r['report']}: cost.json tokens differ from manifest.json")
        pin, pout = PRICE[r["llm_target"]]
        usd = (t["prompt_tokens"] * pin + t["completion_tokens"] * pout) / 1e6
        if abs(usd - c[7]) > 1e-6:
            raise SystemExit(f"report {r['report']}: recomputed engine cost {usd:.6f} != cost.json {c[7]:.6f}")
        pdf = EXPH / f"report-{r['report']}" / "analysis.pdf"
        out.append({
            "report": r["report"], "lang": r["language"], "model": r["llm_target"],
            "model_id": r["token_usage"]["by_model"][0]["model_uid"],
            "prompt": t["prompt_tokens"], "completion": t["completion_tokens"],
            "reasoning": t["reasoning_tokens"], "total": t["total_tokens"], "calls": t["calls"],
            "latency_ms": t["latency_ms"], "usd_engine": usd, "pdf_bytes": pdf.stat().st_size,
            "status": r["status"],
        })
    return out


def fmt_int(n: int) -> str:
    return f"{n:,}".replace(",", " ")  # narrow no-break space thousands separator


def main() -> int:
    bill = billing_rows()
    eng = engine_rows()

    # --- cost of record
    lines = []
    total_usd = 0.0
    rec = {}
    for k in ("o4-mini", "o3"):
        b = bill[k]
        pin, pout = PRICE[k]
        usd_in = b["input"] * pin / 1e6
        usd_out = b["output"] * pout / 1e6
        usd = usd_in + usd_out
        cached_discount = b["cached"] * (pin - (pin / 4)) / 1e6  # published cached rate is 25 % of input rate
        n_reports = 10 if k == "o4-mini" else 2
        rec[k] = {"usd": usd, "usd_in": usd_in, "usd_out": usd_out, "per_report": usd / n_reports,
                  "cached_discount": cached_discount, "n": n_reports, **b}
        total_usd += usd
    per_report_all = total_usd / 12

    # --- engine aggregates
    prim = [e for e in eng if e["model"] == "o4-mini"]
    stretch = [e for e in eng if e["model"] == "o3"]

    def agg(rows, k):
        return sum(r[k] for r in rows)

    eng_o4 = {"prompt": agg(prim, "prompt"), "completion": agg(prim, "completion"),
              "reasoning": agg(prim, "reasoning"), "calls": agg(prim, "calls"), "usd": agg(prim, "usd_engine")}
    eng_o3 = {"prompt": agg(stretch, "prompt"), "completion": agg(stretch, "completion"),
              "reasoning": agg(stretch, "reasoning"), "calls": agg(stretch, "calls"), "usd": agg(stretch, "usd_engine")}
    ratio_out_o4 = rec["o4-mini"]["output"] / eng_o4["completion"]
    ratio_in_o4 = rec["o4-mini"]["input"] / eng_o4["prompt"]
    lat = [e["latency_ms"] for e in eng]
    pdfs = [e["pdf_bytes"] for e in eng]
    assert all(e["status"] == "completed" and e["calls"] == 4 for e in eng)

    md = []
    md.append("# EXP-H cost — single definition of record\n")
    md.append(f"> Generated by `harness/exph_cost.py` on {PRICE_VERIFIED} from `results/_billing/completions_usage_2026-07-14_2026-07-24.csv` "
              f"(rows dated {EXPH_DATE}), `results/exp-h/manifest.json` and `results/exp-h/cost.json`. "
              "Supersedes the three circulating figures ($0.585 engine-based in REPORT.md; “≈ $1.4” in REPORT.md §Deviations and "
              "`reconciliation.md`, whose arithmetic does not reproduce; $1.72 whole-campaign Level-3). See DECISIONS D13.\n")
    md.append("## Definition\n")
    md.append(f"**EXP-H cost of record** = OpenAI usage-export rows dated **{EXPH_DATE}** for **o4-mini** and **o3** "
              "(the only reasoning-model traffic that day; every other 07-24 row belongs to EXP-D E2 / EXP-E, see `reconciliation.md`) "
              f"× published list prices per 1 M tokens (o4-mini ${PRICE['o4-mini'][0]:.2f} in / ${PRICE['o4-mini'][1]:.2f} out; "
              f"o3 ${PRICE['o3'][0]:.2f} in / ${PRICE['o3'][1]:.2f} out; re-verified {PRICE_VERIFIED} at {PRICE_URL}). "
              "Input is priced at the uncached rate; the cached-input discount "
              f"(o4-mini {fmt_int(rec['o4-mini']['cached'])} cached tokens ≈ ${rec['o4-mini']['cached_discount']:.4f}) is ignored as immaterial and conservative. "
              "The account's actually billed amount (service-tier discount) is NOT used: it is account-specific and not reproducible.\n")
    md.append("## Cost of record (billing × list price)\n")
    md.append("| Model | Reports | Billed requests | Input tok | Output tok (incl. reasoning) | USD input | USD output | **USD** | USD / report |")
    md.append("|---|---|---|---|---|---|---|---|---|")
    for k, label in (("o4-mini", "1–10 (primary)"), ("o3", "11–12 (stretch)")):
        r = rec[k]
        md.append(f"| {k} (`{', '.join(sorted(r['model_ids']))}`) | {label} | {r['requests']} | {fmt_int(r['input'])} | {fmt_int(r['output'])} "
                  f"| {r['usd_in']:.4f} | {r['usd_out']:.4f} | **{r['usd']:.3f}** | {r['per_report']:.4f} |")
    md.append(f"| **EXP-H total** | 1–12 | {rec['o4-mini']['requests'] + rec['o3']['requests']} | "
              f"{fmt_int(rec['o4-mini']['input'] + rec['o3']['input'])} | {fmt_int(rec['o4-mini']['output'] + rec['o3']['output'])} | "
              f"{rec['o4-mini']['usd_in'] + rec['o3']['usd_in']:.4f} | {rec['o4-mini']['usd_out'] + rec['o3']['usd_out']:.4f} | "
              f"**{total_usd:.3f}** | {per_report_all:.4f} |\n")
    md.append(f"Article-ready statements: **EXP-H cost ≈ ${total_usd:.2f} (list price)**, i.e. **≈ ${per_report_all:.3f} per report**; "
              f"o4-mini ≈ ${rec['o4-mini']['per_report']:.3f} / report, o3 ≈ ${rec['o3']['per_report']:.3f} / report. "
              "Note: per-report cost of o3 and o4-mini is nearly equal on billing, because o4-mini's billed output is 2.6× the engine count "
              "(see below) — the engine-based ‘o3 ≈ 2× o4-mini’ comparison in REPORT.md is an artefact of the under-count and must not be used.\n")
    md.append("## Engine accounting (per report; distribution only — NOT the cost of record)\n")
    md.append("| # | Lang | Model (deployment) | Prompt tok | Completion tok | of which reasoning | Calls | Latency, s | Engine USD | PDF, kB |")
    md.append("|---|---|---|---|---|---|---|---|---|---|")
    for e in eng:
        md.append(f"| {e['report']} | {e['lang']} | {e['model']} (`{e['model_id']}`) | {fmt_int(e['prompt'])} | {fmt_int(e['completion'])} | "
                  f"{fmt_int(e['reasoning'])} | {e['calls']} | {e['latency_ms'] / 1000:.1f} | {e['usd_engine']:.4f} | {e['pdf_bytes'] / 1000:.0f} |")
    md.append(f"| Σ o4-mini (1–10) | | | {fmt_int(eng_o4['prompt'])} | {fmt_int(eng_o4['completion'])} | {fmt_int(eng_o4['reasoning'])} | {eng_o4['calls']} | | {eng_o4['usd']:.4f} | |")
    md.append(f"| Σ o3 (11–12) | | | {fmt_int(eng_o3['prompt'])} | {fmt_int(eng_o3['completion'])} | {fmt_int(eng_o3['reasoning'])} | {eng_o3['calls']} | | {eng_o3['usd']:.4f} | |")
    md.append(f"| Σ all (1–12) | | | {fmt_int(eng_o4['prompt'] + eng_o3['prompt'])} | {fmt_int(eng_o4['completion'] + eng_o3['completion'])} | "
              f"{fmt_int(eng_o4['reasoning'] + eng_o3['reasoning'])} | {eng_o4['calls'] + eng_o3['calls']} | | {eng_o4['usd'] + eng_o3['usd']:.4f} | |\n")
    md.append(f"Latency per report: min {min(lat) / 1000:.1f} s, max {max(lat) / 1000:.1f} s, mean {sum(lat) / len(lat) / 1000:.1f} s "
              f"(o4-mini mean {sum(e['latency_ms'] for e in prim) / len(prim) / 1000:.1f} s; o3 mean {sum(e['latency_ms'] for e in stretch) / len(stretch) / 1000:.1f} s). "
              f"PDF size {min(pdfs) / 1000:.0f}–{max(pdfs) / 1000:.0f} kB (decimal, as in REPORT.md). All 12 analyses `completed`, 4 LLM calls each.\n")
    md.append("CORRECTION: REPORT.md states o4-mini reasoning tokens as 34 072; the manifest rows sum to "
              f"{fmt_int(eng_o4['reasoning'])} (the per-report values in REPORT.md's source, cost.json, are unchanged). Use the recomputed value.\n")
    md.append("## Engine vs billing reconciliation (2026-07-24, EXP-H only)\n")
    md.append("| Model | Engine calls / prompt / completion | Billing requests / input / output | Ratio input | Ratio output |")
    md.append("|---|---|---|---|---|")
    md.append(f"| o4-mini | {eng_o4['calls']} / {fmt_int(eng_o4['prompt'])} / {fmt_int(eng_o4['completion'])} | "
              f"{rec['o4-mini']['requests']} / {fmt_int(rec['o4-mini']['input'])} / {fmt_int(rec['o4-mini']['output'])} | {ratio_in_o4:.3f} | **{ratio_out_o4:.2f}** |")
    md.append(f"| o3 | {eng_o3['calls']} / {fmt_int(eng_o3['prompt'])} / {fmt_int(eng_o3['completion'])} | "
              f"{rec['o3']['requests']} / {fmt_int(rec['o3']['input'])} / {fmt_int(rec['o3']['output'])} | "
              f"{rec['o3']['input'] / eng_o3['prompt']:.3f} | {rec['o3']['output'] / eng_o3['completion']:.3f} |\n")
    md.append("Reading: o3 reconciles to the token (calls, input, output). For o4-mini the provider billed **41 requests vs 40 engine calls** "
              "(one extra request — most plausibly a JSON-retry or a request whose usage the engine did not fold) and **2.6× the engine's completion tokens**; "
              "input agrees within 1 %. Consequence for the article: cite billing for reasoning-model cost, and treat the engine's `reasoning_tokens` "
              "for o4-mini as a lower bound. The methodological finding (engine under-attributes o4-mini output) is itself reportable.\n")
    md.append("## Service tier / billed amount (context only)\n")
    md.append(f"Service tiers in the 07-24 rows: o4-mini `{', '.join(sorted(rec['o4-mini']['tiers']))}`, o3 `{', '.join(sorted(rec['o3']['tiers']))}`. "
              "The `cost_2026-07-14_2026-07-24.csv` export posts one whole-window amount ($5.8177 on 2026-07-24) for ALL campaign traffic and cannot be attributed to EXP-H; "
              "it is not used.\n")
    md.append("## Files\n")
    md.append("- `completions_usage_2026-07-14_2026-07-24.csv` — daily usage per model (the two 07-24 reasoning-model rows are the cost of record). "
              "⚠ Contains account identifiers (project/org/user/API-key ids): keep in this private repo; **scrub or omit before the public artifacts repo (Task 12)**.")
    md.append("- `reconciliation.md` — the sibling campaign's whole-window reconciliation (copied verbatim, provenance header added).")
    md.append("- `../exp-h/cost.json`, `../exp-h/manifest.json` — engine token accounting per report.")
    OUT.write_text("\n".join(md) + "\n", encoding="utf-8")

    print(f"wrote {OUT.relative_to(ROOT)}")
    print(f"EXP-H cost of record: ${total_usd:.4f} total; o4-mini ${rec['o4-mini']['usd']:.4f} ({rec['o4-mini']['per_report']:.4f}/report); "
          f"o3 ${rec['o3']['usd']:.4f} ({rec['o3']['per_report']:.4f}/report); all ${per_report_all:.4f}/report")
    print(f"engine-based (cost.json): o4-mini ${eng_o4['usd']:.4f}, o3 ${eng_o3['usd']:.4f}, total ${eng_o4['usd'] + eng_o3['usd']:.4f}")
    print(f"o4-mini billing/engine output ratio {ratio_out_o4:.3f}; input ratio {ratio_in_o4:.3f}")
    print(f"latency s: min {min(lat) / 1000:.1f} max {max(lat) / 1000:.1f} mean {sum(lat) / len(lat) / 1000:.1f}; pdf kB {min(pdfs) / 1000:.0f}-{max(pdfs) / 1000:.0f}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
