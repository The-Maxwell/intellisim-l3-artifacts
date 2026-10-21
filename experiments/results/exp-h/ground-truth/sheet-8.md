# EXP-H ground-truth sheet - report 8

**Source run:** EXP-F wildfire, super-critical density p=0.70 (rep1)
**Sim ID:** `6a634dbf4ccaa6983a640151` - **Language:** en - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Fire spreads through a forest of trees."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Near-complete burn - almost the entire forest burns (super-critical regime). | Burned fraction (D34) = 0.9925 for this rep; sweep mean at p=0.70 = 0.976 +/- 0.008. Source: `exp-f/intellisim/sweep-61/raw/6a634dbf4ccaa6983a640151-p0.70-rep1-steps.csv.gz` + `exp-f/intellisim/sweep-61/metrics.json` (p=0.70). Data-supported: 0.96-0.99. |
| 2 | Burn-through across the grid - fire reaches and burns the far (right) edge (percolation). | percolated = True (ash present in the rightmost column x=60); sweep perc_prob at p=0.70 = 1.0. Source: rep1 gz + metrics.json. Data-supported: percolation present. |
| 3 | Sustained spread over many steps - the fire front propagates across the whole grid before dying. | Run length 85 steps (vs 10 at p=0.50). Source: rep1 gz. Data-supported: long burn (>> sub-critical), front crosses the grid. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** en.

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
