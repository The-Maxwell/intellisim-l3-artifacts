# EXP-H ground-truth sheet - report 3

**Source run:** EXP-B Test 12 bottleneck, 150 agents, 2 rooms + corridor (rep1, seed 1201)
**Sim ID:** `6a5bc249c44d001ef3af684d` - **Language:** en - **llm_target:** o4-mini
**Scenario hint (context, non-leading):** "150 people move out of a building section consisting of two rooms connected by a corridor."
**Pre-filled:** 2026-07-24 (committed BEFORE any analysis generated - blinding/pre-registration evidence).
**Rubric (per phenomenon):** 2 = explicitly identified AND quantitatively correct (within data-supported range) - 1 = partial/qualitative or imprecise-quantitative - 0 = missed/wrong.

| # | Phenomenon (pre-filled) | Ground truth - measured value(s), data-supported range, source path |
|---|---|---|
| 1 | Congestion localized in the upstream room (Room 1), NOT the downstream room (Room 2) - the queue builds at the first door, not the exit. | Peak occupancy zone-1 (Room-1 door) 37.2 (35-40) vs zone-2 (Room-2 exit) 12.7 (12-14), ratio 2.9, non-overlapping across 10 reps. Source: `exp-b/test-12/REPORT.md`. Data-supported: upstream peak >= 2x downstream. |
| 2 | The Room-1 door / corridor is the limiting (bottleneck) element - sustained upstream queue; a small number of agents wedge at the pinch points. | Zone-1 occupancy >=10 for ~34.5 steps (sustained); exactly 3 agents wedge at fixed geometric pinch points every rep. Source: `exp-b/test-12/REPORT.md`, rep1 CSV. Data-supported: sustained upstream congestion + limited throughput. |
| 3 | Clearance time (measured) - 147/150 (98%) evacuate by ~step 98; 3 persistent stragglers keep the run to max_steps (599). | rep1: population <=3 at step 98; final population 3. Source: `exp-b/test-12/raw/6a5bc249c44d001ef3af684d-rep1-steps.csv`. Data-supported: 98% egress by ~90-100 steps; 3 stragglers. |

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
