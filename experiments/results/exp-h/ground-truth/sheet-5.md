# EXP-H ground-truth sheet - report 5

**Source run:** EXP-E archetype_decision run (rep301, seed 301) - LLM makes per-archetype decisions
**Sim ID:** `6a6380584ccaa6983ac7ea1c` - **Language:** en - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "People are evacuating a room through one door; some agent decisions during the run are made by a language model."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Egress is slower and more spread than the rules-only reference - the LLM-driven decisions lengthen and disperse evacuation. | This run T_total = 98 steps (29.4 s) vs rules-only `none` 46 steps (13.8 s). Per-agent egress (condition mean) 10.8 +/- 6.4 s vs none 6.7 +/- 3.3 s (~2x spread). Source: `exp-e/stats.json` (archetype_decision vs none), `exp-e/archetype_decision/run-301/metrics.json` (steps=98). Data-supported: T ~87-98 steps; egress SD elevated ~2x baseline. |
| 2 | Non-degenerate decision-label mix INCLUDING hesitation (wait_briefly) - all four menu options appear, not just proceed. | rep301 label counts: proceed_to_exit 441 (61.8%), follow_crowd 178 (24.9%), wait_briefly 55 (7.7%), seek_alternate_route 40 (5.6%); 714 decision events. Source: `exp-e/archetype_decision/run-301/metrics.json` `label_counts`. Data-supported: >=3 distinct labels present, wait_briefly > 0. |
| 3 | Evacuation nonetheless completes - all agents exit; run terminates condition_met (population -> 0), not by timeout. | termination_reason = condition_met at step 98; all agents exit. Source: `exp-e/archetype_decision/run-301/metrics.json` (`termination_reason`,`steps`). Data-supported: full egress, condition_met. |

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
