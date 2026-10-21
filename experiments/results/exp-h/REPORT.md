> **Provenance:** verbatim copy of `results/exp-h/REPORT.md` from `../intellisim.experiments` @ `a9694df`, copied 2026-09-08 for the Level-3 article campaign (`intellisim.experiments.l3`). Paths inside refer to the sibling repo; here the same material lives under `results/exp-h/`, `results/_billing/`, `experiments/`. Do not edit; corrections are recorded in `results/_article/KEY-NUMBERS.md`.

# EXP-H — Level-3 analysis validity against known ground truth: REPORT

**Date:** 2026-07-24 · **Protocol:** `experiments/30-evacuation/36-analysis-eval-protocol.md` · **Verdict: BOTH pre-registered criteria PASS — 54/60 phenomenon points (90%); 7 factual errors across 10 primary reports (mean 0.7/report).**

- Engine: core `6bb5bd9`. Level-3 pipeline: deterministic evidence assembly → 3 sectioned reasoning calls (dynamics / spatial / emergence) → 1 synthesis call → Maroto-v2 PDF (GridFS). `llm_target` = **o4-mini** (10 primary) + **o3** (2 stretch). Reasoning models justified per D3 (Level-3 = a scientifically-necessary LLM use).
- **Blinding gate:** all 10 ground-truth sheets + the run manifest were committed at **`23ff33d`** BEFORE the first analysis was generated (the first `POST /analyses` is `6a63a240…`, created after that commit). Sheets score against the **measured/exported data**, not external SFPE/analytic bands (§2.1).
- 12/12 analyses completed cleanly (4 LLM calls each, all 3 sections + synthesis, conclusions + recommendations populated, **0 warnings**, PDF renders 102–155 KB). uk Cyrillic verified end-to-end (JSON + PDF, no tofu).
- Reproducible: `results/exp-h/manifest.json` (run list + verbatim request bodies + analysis IDs), `report-N/{request.json, report.json, analysis.pdf, scoring.md}`, `cost.json`.

## Report set (10 primary + 2 stretch)

| # | Source run (sim ID) | Lang | Target | Scores P1/P2/P3 | Σ/6 | Errors | Halluc | Useful |
|---|---|---|---|---|---|---|---|---|
| 1 | EXP-C room evac, median rep (`6a5bbcaa…207c`) | en | o4-mini | 2 / 2 / 2 | **6** | 0 | 0 | y |
| 2 | EXP-C room evac, fastest rep (`6a5bbcc1…c10e`) | uk | o4-mini | 2 / 2 / 1 | **5** | 1 | 0 | y |
| 3 | EXP-B T12 bottleneck, 150 (`6a5bc249…684d`) | en | o4-mini | 2 / 2 / 2 | **6** | 0 | 0 | y |
| 4 | EXP-B T4 wide-opening, N=300 (`6a5bc463…f824`) | uk | o4-mini | 2 / 1 / 0 | **3** | 2 | 1 | y |
| 5 | EXP-E archetype_decision (`6a638058…ea1c`) | en | o4-mini | 2 / 2 / 1 | **5** | 1 | 0 | y |
| 6 | EXP-E individual_decision (`6a63810f…1367`) | uk | o4-mini | 2 / 1 / 2 | **5** | 1 | 0 | y |
| 7 | EXP-F wildfire p=0.50 (`6a610ff2…b995`) | uk | o4-mini | 2 / 2 / 2 | **6** | 2 | 0 | y |
| 8 | EXP-F wildfire p=0.70 (`6a634dbf…0151`) | en | o4-mini | 2 / 2 / 2 | **6** | 0 | 0 | y |
| 9 | EXP-G epidemic med, rep1 (`6a636be6…edd5`) | en | o4-mini | 2 / 2 / 2 | **6** | 0 | 0 | y |
| 10 | EXP-G epidemic med, rep8 (`6a636c0c…976a`) | uk | o4-mini | 2 / 2 / 2 | **6** | 0 | 0 | y |
| **Σ primary (1–10)** | | | | | **54 / 60** | **7** | **1** | **10/10** |
| 11 | EXP-C median rep, o3 re-run (`6a5bbcaa…207c`) | en | o3 | 2 / 2 / 2 | **6** | 0 | 0 | y |
| 12 | EXP-C fastest rep, o3 re-run (`6a5bbcc1…c10e`) | uk | o3 | 2 / 2 / 2 | **6** | 0 | 0 | y |

Per-report evidence quotes + error transcripts: `report-N/scoring.md`. Ground-truth sheets: `ground-truth/sheet-N.md`.

## Verdicts vs pre-registered criteria (§3)

- **Criterion 1 — phenomenon points ≥ 70% (≥ 42/60) — PASS: 54/60 = 90%.** 7 of 10 primary reports scored a perfect 6/6; the mean is 5.4/6. Nine of ten reports scored ≥5/6.
- **Criterion 2 — factual errors mean ≤ 1/report (≤ 10 total) — PASS: 7 total, mean 0.7/report.** No report exceeded 2 errors. Every error is transcribed verbatim in the per-report `scoring.md`.
- **Criterion 3 (descriptive, no threshold):** hallucinated phenomena = **1** across 10 primary reports (a single fundamental-diagram over-interpretation in report 4); usefulness = **10/10** (every report gave concrete, actionable, data-relevant recommendations); en/uk and o4-mini/o3 comparisons below.

**This directly answers the LLM-judge-circularity critique [Larooij2025]:** reports were scored against phenomena whose ground truth was *independently measured in the source experiments* (EXP-B/C/E/F/G) and committed before generation — not against another model's opinion.

## What the analyses got right (highlights)

- **Exact quantitative reproduction.** Report 5 reproduced the EXP-E archetype decision-label counts **exactly** (441 proceed / 178 follow / 40 alternate / 55 wait). Report 9 reported the epidemic peak **22 infected at step 60** and final size **37%** — both exact matches to the manifest. Report 8 reported **97.8%** burned + percolation for the super-critical fire.
- **Correct regime discrimination.** The pipeline cleanly separated sub-critical (report 7: fire dies by step 10, ~6–8% burned, no burn-through) from super-critical (report 8: ~98% burned, percolation, 85-step front) wildfire, and single-peak self-limiting epidemics (reports 9/10) with correct SIR monotonicity.
- **Correct bottleneck localisation.** Report 3 localised congestion upstream (Room 1) vs the cleared downstream room and identified the 3-agent persistent straggler cluster + ~step-100 clearance.
- **The EXP-C fast-drain deviation was NOT hallucinated away.** Reports 1/2/11/12 reported egress in **~44–46 steps** (the measured value), never the SFPE ~110 s figure — i.e. the analysis read the data, not the textbook expectation.

## Where the analyses were weak (honest)

- **Report 4 (EXP-B T4, uk) — 3/6, the lowest.** The T4 ground truth (a fundamental diagram, rising branch + absence of saturation) is an **ensemble** result across N50/150/300 that is *not observable in a single fixed-N run*, and the run's within-time flow profile is dominated by travel-time-to-opening. The report over-interpreted this as a density–flow optimum ("max ≈6.5 agents/step at ~160 agents") and mis-attributed the low early flow to high-density congestion (the engine has no speed–density coupling, per EXP-B/C). Per protocol §3 this is **data, not a protocol failure** — and it usefully bounds what single-run Level-3 analysis can infer.
- **Absolute time units.** Report 2 (o4-mini) appended a wrong "≈57 s" to a correct 43-step egress (the model carries no declared time scale; the engine convention is 0.3 s/step ⇒ ~13 s). The o3 re-run (report 12) **correctly** converted 0.3 s/step → 13.5 s and inferred 0.4 m/cell — the sharpest o4-mini-vs-o3 difference.
- **Minor slips:** report 5 implied a permanent straggler where the run reached `condition_met` (all evacuated); report 6 said "single-cell door" in its summary (correct "three-cell" elsewhere); report 7 reversed the fire-front direction in one line and carried a 4.4%-vs-138-ash internal inconsistency.

## en vs uk

| Split | Reports | Σ points | Mean/6 | Factual errors |
|---|---|---|---|---|
| en | 1, 3, 5, 8, 9 | 29 / 30 (96.7%) | 5.8 | 1 |
| uk | 2, 4, 6, 7, 10 | 25 / 30 (83.3%) | 5.0 | 6 |

Ukrainian Cyrillic rendered correctly in every uk report (JSON + PDF). The en/uk gap is driven mostly by report 4 (the hard T4 single-run case, which happened to fall on uk) and the report-2 time-unit slip; it is **not** a Cyrillic-pipeline defect. Small n — reported descriptively.

## o4-mini vs o3 (stretch)

| Model | Pair (reports) | Σ points | Errors | Notable |
|---|---|---|---|---|
| o4-mini | 1, 2 | 11 / 12 | 1 | fabricated "≈57 s" absolute time (report 2) |
| o3 | 11, 12 | 12 / 12 | 0 | correct unit inference (0.3 s/step→13.5 s; 0.4 m/cell); richer spatial quantification (local door density ~0.67/cell) |

On the identical two runs, **o3 scored higher (12/12 vs 11/12) with zero factual errors**, chiefly by getting the physical unit conversions right where o4-mini guessed. o3 cost ~1.9× the o4-mini per-report token spend.

## Cost (measured from engine token accounting)

Pricing used (per 1M in/out, `TODO-VERIFY` at execution date, recorded in `manifest.json`): **o4-mini $1.10 / $4.40**, **o3 $2.00 / $8.00**. Reasoning tokens are a subset of completion (already priced as output).

| Set | Reports | Prompt tok | Completion tok (incl. reasoning) | Cost (list) |
|---|---|---|---|---|
| Primary (o4-mini) | 1–10 | 130 755 | 60 917 (34 072 reasoning) | **$0.412** |
| Stretch (o3) | 11–12 | 22 001 | 16 176 (9 792 reasoning) | **$0.173** |
| **Total** | 1–12 | 152 756 | 77 093 | **$0.585** |

Per-report mean ≈ $0.041 (o4-mini) / $0.087 (o3). Pilot projection (report 1: $0.035 ×12) held. Trivially within any campaign budget; Level-3 spend is separate from the EXP-E/D-E2 $15 cap (D32). Full breakdown: `cost.json`.

## Deviations (§7)

- **Report 4 substitution NOT triggered:** EXP-B Test 4 was executed (`results/exp-b/test-4/`), so the §1.1 "substitute a second Test-12 rep" clause did not apply; report 4 uses a Test-4 N=300 run. Its ground-truth sheet was adapted (per §2.1) to the single-run-observable data. Logged here; no DECISIONS row needed (within-protocol).
- **Stretch pair added (reports 11–12, o3):** executed per §4/§1.1 (budget permitting) — cost was negligible ($0.17). Logged.
- **Cost pricing is `TODO-VERIFY`** (o4-mini/o3 list rates at execution date). **Billing cross-check (`results/_billing/`, 2026-07-24):** OpenAI's export confirms EXP-H's o3 tokens exactly (22 001 in / 16 176 out) but shows the engine **under-counts o4-mini OUTPUT ~2.6×** (60.9k engine vs **160.9k billed** — reasoning-model output not fully folded by the engine). So the true o4-mini list cost is ~**$0.85** (132k in + 161k out), not the ~$0.41 estimated here from engine tokens; EXP-H list-price is therefore higher (~$1.4 for the 07-24 o4-mini+o3 subset) than the engine-based $0.585. **Verdicts are unaffected** (cost is descriptive), but cite billing, not engine, for reasoning-model cost. See `_billing/reconciliation.md`.
- **Ground-truth sheet hints transliterated** in the sheet headers (author reference is English); the *actual* uk request bodies use Cyrillic (verbatim in `manifest.json` / `report-N/request.json`).

## Artifact accounting (D20)

EXP-H created **12 analyses** (`simulation_analyses`) + 12 GridFS PDFs on 10 pre-existing source sims (none created, none deleted). Analysis IDs `6a63a240…` … `6a63a4fe…` recorded in `manifest.json`; to be registered in `results/ARTIFACTS.md`. Source sims (EXP-B/C/E/F/G) untouched (D20).
