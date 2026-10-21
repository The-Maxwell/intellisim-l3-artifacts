# EXP-H scoring - report 5

Meta: sim_id=6a6380584ccaa6983ac7ea1c analysis_id=6a63a3744ccaa6983ac9831f lang=en llm_target=o4-mini generated=2026-07-24T17:40:04.948Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-5.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a room through one door; some agent decisions during the run are made by a language model. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Low seek_alternate_route rate (40 executions) → lack of route diversification → persistent bottleneck → prolonged tail-phase evacuation." + "All 100 agents evacuated by step 96; first 50 exited within 40 steps, last 20 took 56 steps." + "The final 10% of agents require over one-third of total evacuation time" (prolonged, dispersed egress attributed to the LLM decision mix; total duration 96-98 steps within data-supported 87-98) |
| 2 | 2 | "a strong LLM bias toward “decide_proceed_to_exit” (441 calls, 54%) and “decide_follow_crowd” (178 calls, 22%) ... “decide_seek_alternate_route” (40 calls, 5%) and “decide_wait_briefly” (55 calls, 7%)" (all four labels present with exact counts 441/178/40/55 matching the data; wait_briefly > 0) |
| 3 | 1 | "All 100 agents evacuated by step 96" / "exit_when_at_door fired exactly 100 times (once per agent)" - but contradicted in the same report by "Simulation completes at step 98 with one straggler remaining." (full egress stated, yet a residual agent is also asserted; termination reason not stated) |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'Simulation completes at step 98 with one straggler remaining.' (repeated as 'The simulation ends at step 98 with one straggler still inside' and 'Persistent straggler: one agent remains past step 96 despite bulk clearance.') - data: all agents exit, termination_reason = condition_met (population -> 0) at step 98; there is no agent left inside at the end of the run.

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Widen the exit or add additional doors in the model to assess effects on throughput and tail-phase duration."

Language/format notes: English as requested; all three sections + executive summary + conclusions + recommendations present. Format artifacts: raw ANSI escape sequences ([1m ... [0m) embedded around rule names in the dynamics section; a stray "–4" (en-dash for minus) in "peaked at –4 agents". Internal wobbles (not counted as errors): label shares are given as percentages of all 814 rule executions (54.2/21.9/4.9/7%) rather than of the 714 LLM decision events (61.8/24.9/5.6/7.7%) - the denominator is stated once ("441 of 814 rule executions") but the emergence section presents them as shares of "Agent decisions mediated by the LLM"; evacuation completion is given as both step 96 and step 98. Step-level plateaus/burst steps (3, 9, 26, 50, 56, 24, 40) and the 3-cell door / 38x38 room are not verifiable from the sheet and are treated as unknown.
