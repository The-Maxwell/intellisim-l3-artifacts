# EXP-H scoring - report 5

Meta: sim_id=6a6380584ccaa6983ac7ea1c analysis_id=6a63a3744ccaa6983ac9831f lang=en llm_target=o4-mini generated=2026-07-24T17:40:04.948Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-5.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a room through one door; some agent decisions during the run are made by a language model. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "a pronounced tail-phase slowdown where the final 9 agents exit at ~0.25 agents/step over 36 steps ... completes at step 98" |
| 2 | 2 | "decide_proceed_to_exit (441 calls, 54%) ... decide_follow_crowd (178, 22%) ... decide_seek_alternate_route (40, 5%) ... decide_wait_briefly (55, 7%)" |
| 3 | 1 | "The simulation ends at step 98 with one straggler still inside" |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'The simulation ends at step 98 with one straggler still inside' / 'a persistent straggler beyond step 96' - the run terminated condition_met (population->0): all agents evacuated by step 98, no permanent straggler

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Rebalance LLM decision probabilities to increase 'seek_alternate_route' and decrease crowd-following bias"

Language/format notes: en. Reproduces the rep-301 decision-label counts EXACTLY (441/178/40/55) and identifies all four labels incl. wait_briefly (P2=2). P1=2: long spread-out egress to step 98 captured. P3=1: near-complete evacuation recognised but wrongly implies a permanent straggler (run was condition_met).
