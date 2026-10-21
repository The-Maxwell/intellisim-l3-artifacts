# EXP-H ground-truth sheet - report 9

**Source run:** EXP-G epidemic, medium-transmissibility (med point beta=0.18, rep1 seed 8501)
**Sim ID:** `6a636be64ccaa6983aafedd5` - **Language:** en - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "An infectious disease spreads in a population of moving agents."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | A single epidemic peak (one wave) - infected count rises to a peak then declines to extinction. | Peak infected I = 22 at step 60; single hump; epidemic ends (I->0). Source: `exp-g/intellisim/manifest.json` (med rep1: peak_infected 22, peak_step 60, steps 228) + `exp-g/intellisim/point-med/curves.jsonl`. Data-supported: single peak, height ~18-26, timing ~50-70. |
| 2 | Final size (measured) - a modest fraction of the population is ever infected; well BELOW the well-mixed analytic bound (spatial suppression). | Final size = 74/200 = 0.37 (final_recovered 74). Well-mixed analytic z at R0~=2.1 = 0.822 (upper bound; measured < analytic due to spatial mixing). Source: `exp-g/intellisim/manifest.json` (med rep1 final_size 0.37) + `exp-g/stats.json` (analytic_z med 0.822). Data-supported: final size ~0.30-0.40, < 0.822. |
| 3 | SIR monotonicity - susceptible non-increasing, recovered non-decreasing over time. | S(t) monotone down, R(t) monotone up (SIR invariant). Source: rep1 step series (`exp-g/intellisim/point-med/raw/med-rep1-steps.csv.gz`). Data-supported: strict monotonicity of S and R. |

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
