# EXP-H ground-truth sheet - report 1

**Source run:** EXP-C room evacuation, median-T_drain rep (rep3, seed 103)
**Sim ID:** `6a5bbcaac44d001ef3ac207c` - **Language:** en - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "People are evacuating a single room through one door."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Door congestion / queue at the single exit - agents accumulate at the one door; egress throughput is capped by door capacity (~2.2 agents/step ~= 7.3 P/s). | Discharge ~= 100 agents over ~45 steps => ~2.2 agents/step steady. Queue/cluster forms at the door region. Source: `exp-c/raw/6a5bbcaac44d001ef3ac207c-rep3-steps.csv` (agent x,y near door), `exp-c/metrics.json` rep3 (Fc 11.1, Fs 11.7). Data-supported: throughput 2.0-2.4 agents/step; visible door queue. |
| 2 | Flow-limited, approximately constant-rate egress - evacuated_count rises ~linearly, population falls ~linearly (no early plateau / no acceleration). | `evacuated_count`: 0->100 roughly linearly from step ~2 to step 45 (98 at step 43, 99 at 44, 100 at 45). Source: `exp-c/raw/6a5bbcaac44d001ef3ac207c-rep3-steps.csv` `evacuated_count` column. Data-supported: monotone near-linear discharge. |
| 3 | Total egress time ~= 13.5 s (45 steps) - the MEASURED completion time. | T_total = 13.5 s (45 steps @ 0.3 s/step); T_drain 12.9 s. Ensemble range 13.2-14.1 s (44-47 steps). Source: `exp-c/metrics.json` rep3 + all reps. NOTE: this is ~6.6x FASTER than the SFPE ~110 s reference (documented engine-class deviation, D29). Score against the DATA (~13.5 s). Reporting the model's egress as ~110 s = factual error vs data. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** en. Expect all sections + PDF + conclusions.

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
