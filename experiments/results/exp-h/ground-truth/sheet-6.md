# EXP-H ground-truth sheet - report 6

**Source run:** EXP-E individual_decision run (rep301, seed 301) - LLM decides per individual agent
**Sim ID:** `6a63810f4ccaa6983ac81367` - **Language:** uk - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "Lyudy evakuyuyut'sya z prymishchennya cherez odni dveri; chastynu rishen' ahentiv pid chas vykonannya ukhvalyuye movna model'."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Egress time/spread relative to the rules-only reference - here egress is ~= the baseline (NOT elevated): individual per-agent LLM decisions collapse onto the rules-only optimum. | This run T_total = 46 steps = rules-only `none` 46; per-agent egress (condition mean) 6.76 +/- 3.25 s ~= none 6.73 +/- 3.28 s. Source: `exp-e/stats.json` (individual_decision vs none), `exp-e/individual_decision/run-301/metrics.json` (steps=46). Data-supported: T ~= 46 steps, egress ~= baseline. (Reporting individual as *slower/wider* than baseline = factual error vs data.) |
| 2 | Decision-label mix is degenerate - essentially all decisions are proceed_to_exit; no hesitation. | rep301: proceed_to_exit 494 of 494 measured decision events (others 0). Condition-wide 2456 proceed vs 2 seek_alternate. Source: `exp-e/individual_decision/run-301/metrics.json` `label_counts`. Data-supported: >=99% proceed_to_exit; wait_briefly ~= 0. |
| 3 | Evacuation completes - all agents exit; condition_met (population -> 0). | termination_reason = condition_met at step 46. Source: `exp-e/individual_decision/run-301/metrics.json`. Data-supported: full egress, condition_met. |

**Scoring note:** the acceptable range below is the DATA-supported range (not any external SFPE/analytic band). Score against what the exported data shows.

**Language/format:** uk (hint transliterated here; request uses Cyrillic). Verify Cyrillic. NOTE: honest signature is *collapse to baseline*, not elevated spread.

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
