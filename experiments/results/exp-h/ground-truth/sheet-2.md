# EXP-H ground-truth sheet - report 2

**Source run:** EXP-C room evacuation, fastest-T_drain rep (rep18, seed 118)
**Sim ID:** `6a5bbcc1c44d001ef3acc10e` - **Language:** uk - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Lyudy evakuyuyut'sya z odnoho prymishchennya cherez odni dveri."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Door congestion / queue at the single exit - throughput capped by the one door (~2.3 agents/step). | Discharge ~= 100 agents over 44 steps => ~2.3 agents/step. Source: `exp-c/raw/6a5bbcc1c44d001ef3acc10e-rep18-steps.csv`, `exp-c/metrics.json` rep18 (Fc 11.1, Fs 11.7). Data-supported: 2.0-2.5 agents/step; door queue. |
| 2 | Flow-limited, approximately constant-rate egress. | `evacuated_count` 0->100 near-linearly (98@42, 99@43, 100@44). Source: rep18 CSV `evacuated_count`. Data-supported: monotone near-linear. |
| 3 | Total egress time ~= 13.2 s (44 steps) - MEASURED. | T_total 13.2 s (44 steps), T_drain 12.0 s (fastest of 30). Ensemble 13.2-14.1 s. Source: `exp-c/metrics.json` rep18. NOTE: ~8x faster than SFPE ~110 s (D29) - score vs DATA. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** uk (hint shown transliterated here; the actual request uses Cyrillic). Verify Cyrillic renders in JSON + PDF (no tofu), sections/conclusions present.

---
## Scoring (filled at scoring time, after the report exists)

Meta: analysis_id=... generated=... scorer=Claude (author) ... scored=...

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | | |
| 2 | | |
| 3 | | |

Factual errors (verbatim, one per line + what the data says): ... count = X
Hallucinated phenomena (verbatim + why unsupported): ... count = Y
Actionable recommendations: y/n - quote: "..."
Language/format notes (uk rendering, PDF/Cyrillic, section completeness, warnings shown): ...
