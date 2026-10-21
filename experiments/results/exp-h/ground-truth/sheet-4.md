# EXP-H ground-truth sheet - report 4

**Source run:** EXP-B Test 4 wide-opening flow, N=300 (rep1, seed 401)
**Sim ID:** `6a5bc463c44d001ef3b8f824` - **Language:** uk - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Lyudy prokhodyat' kriz' shyrokyy otvir za riznoyi shchil'nosti natovpu."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Sustained, high throughput through the wide (4 m / 10-cell) opening - the crowd discharges continuously at a high, roughly steady rate (~3.6 agents/step). | 297/300 exit by step 83 => ~3.6 agents/step; population 300->223->92->19->4 at steps 0/20/40/60/80. Source: `exp-b/test-4/N300/raw/6a5bc463c44d001ef3b8f824-rep1-steps.csv`. Data-supported: 3.3-3.9 agents/step sustained. |
| 2 | Specific flow rises with crowd density (fundamental-diagram rising branch) - flow increases with density rather than a fixed ceiling. | Fs rises with density; matches RiMEA rising branch within ~10-15% up to ~1.5 P/m^2. Source: `exp-b/test-4/fundamental-diagram.json`, `exp-b/test-4/REPORT.md`. Data-supported: monotone-rising Fs on the low-moderate branch. |
| 3 | Absence of high-density saturation - flow does NOT plateau at high density (documented engine limitation: no speed-density coupling). | Beyond ~2 P/m^2 Fs keeps rising ~linearly (3.0-5.7 at D 3-6), no plateau. Source: `exp-b/test-4/fundamental-diagram.json`. Data-supported: no saturation. Asserting a classic congestion slowdown/plateau = factual error vs data. 3 stragglers -> max_steps 899. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** uk (hint transliterated here; request uses Cyrillic). Verify Cyrillic. NOTE: this run's honest signature is *sustained/unsaturated* flow, not a slowdown.

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
