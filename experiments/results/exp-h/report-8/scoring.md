# EXP-H scoring - report 8

Meta: sim_id=6a634dbf4ccaa6983a640151 analysis_id=6a63a4084ccaa6983ac98328 lang=en llm_target=o4-mini generated=2026-07-24T17:42:32.071Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-8.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: Fire spreads through a forest of trees. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "ash accumulated monotonically to 2,547 (97.8% of initial trees) ... consumes ~98% of trees" |
| 2 | 2 | "near-complete percolation through the forest ... complete penetration through dense zones" |
| 3 | 2 | "burning counts falling to zero around step 85 ... advanced in a near-planar front" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Vary initial tree density around the percolation threshold to identify critical densities for sustained wave propagation"

Language/format notes: en. All 3 super-critical phenomena correct: near-complete burn (97.8% ~= 99.25%), percolation/burn-through, sustained 85-step front. Denominator noise (2605 vs 2521) does not change magnitude.
