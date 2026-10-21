# EXP-H scoring - report 3

Meta: sim_id=6a5bc249c44d001ef3af684d analysis_id=6a63a3254ccaa6983ac98319 lang=en llm_target=o4-mini generated=2026-07-24T17:38:45.776Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-3.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: 150 people move out of a building section consisting of two rooms connected by a corridor. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "the corridor and downstream Room 2 quickly clear, while the 1.2 m doors upstream sustain high-density arch-like queues" |
| 2 | 2 | "door width is the critical parameter controlling throughput and residual congestion patterns" |
| 3 | 2 | "147 of 150 agents evacuated by step 100; 3 remain in a persistent congested cluster ... 3 persons remain from step 100 to 599" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Perform a parameter sweep on door widths (e.g., 1.2 m, 1.5 m, 2 m) to quantify throughput gains"

Language/format notes: en. Correctly localises congestion upstream (Room 1) vs cleared downstream (Room 2) and identifies the 3-agent persistent straggler cluster + clearance ~step 100 exactly.
