# EXP-H scoring - report 9

Meta: sim_id=6a636be64ccaa6983aafedd5 analysis_id=6a63a43f4ccaa6983ac9832b lang=en llm_target=o4-mini generated=2026-07-24T17:43:27.088Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-9.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: An infectious disease spreads in a population of moving agents. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "a classic single-peaked epidemic ... 22 active cases (11% of the population) at step 60 before recoveries overtook transmission" |
| 2 | 2 | "74 agents recovered (37% infection penetration) and 126 remained susceptible, demonstrating incomplete saturation" |
| 3 | 2 | "susceptibles dropped from 194 to 165 ... recovered reached 30 ... 74 (monotone S down, R up)" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Adjust infection radius or transmission probability to identify thresholds for sustained secondary waves"

Language/format notes: en. Peak 22 @ step 60 EXACT; final size 37% (74/200) EXACT; S non-increasing / R non-decreasing shown. The 'minor secondary waves' are hedged and consistent with the stochastic spatial curve (not a hallucination).
