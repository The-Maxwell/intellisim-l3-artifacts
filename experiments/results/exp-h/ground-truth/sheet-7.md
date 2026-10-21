# EXP-H ground-truth sheet - report 7

**Source run:** EXP-F wildfire, sub-critical density p=0.50 (rep1)
**Sim ID:** `6a610ff2ff82d466bf2db995` - **Language:** uk - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Vohon' poshyryuyet'sya lisom."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | The fire dies out quickly / does not sustain - burning ends after few steps (sub-critical regime). | Fire extinguishes by step 10 (run length 10 steps). Source: `exp-f/intellisim/sweep-61/raw/6a610ff2ff82d466bf2db995-p0.50-rep1-steps.csv.gz`. Data-supported: short burn (<= ~15 steps). |
| 2 | Small final burned fraction - only a small share of the forest burns. | Burned fraction (non-left-column ash / non-left-column trees, D34) = 0.0594 for this rep; sweep mean at p=0.50 = 0.098 +/- 0.044. Source: rep1 gz + `exp-f/intellisim/sweep-61/metrics.json` (p=0.50). Data-supported: 0.04-0.15. |
| 3 | No burn-through to the far (right) edge - fire does not percolate across the grid (below p_c ~= 0.59). | percolated = False (no ash in the rightmost column x=60); sweep perc_prob at p=0.50 = 0.0. Source: rep1 gz + `exp-f/intellisim/sweep-61/metrics.json`. Data-supported: no percolation. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** uk (hint transliterated here; request uses Cyrillic). Verify Cyrillic.

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
