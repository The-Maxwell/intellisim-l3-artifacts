# EXP-H ground-truth sheet - report 10

**Source run:** EXP-G epidemic, medium-transmissibility 2nd rep (med point beta=0.18, rep8 seed 8508)
**Sim ID:** `6a636c0c4ccaa6983ab1976a` - **Language:** uk - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Infektsiyne zakhvoryuvannya poshyryuyet'sya v populyatsiyi rukhomykh ahentiv."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | A single epidemic peak (one wave). | Peak infected I = 21 at step 51; single hump; epidemic ends. Source: `exp-g/intellisim/manifest.json` (med rep8: peak_infected 21, peak_step 51, steps 174). Data-supported: single peak, height ~17-25, timing ~40-60. |
| 2 | Final size (measured) - modest fraction infected, below the well-mixed analytic bound. | Final size = 66/200 = 0.33 (final_recovered 66). Well-mixed analytic z at R0~=2.1 = 0.822. Source: `exp-g/intellisim/manifest.json` (med rep8 final_size 0.33) + `exp-g/stats.json`. Data-supported: ~0.28-0.38, < 0.822. |
| 3 | SIR monotonicity - S non-increasing, R non-decreasing. | S down monotone, R up monotone. Source: `exp-g/intellisim/point-med/raw/med-rep8-steps.csv.gz`. Data-supported: strict monotonicity. |

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
