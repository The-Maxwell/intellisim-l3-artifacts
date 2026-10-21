# EXP-H scoring - report 1

Meta: sim_id=6a5bbcaac44d001ef3ac207c analysis_id=6a63a2404ccaa6983ac98313 lang=en llm_target=o4-mini generated=2026-07-24T17:34:56.228Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-1.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a single room through one door. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "A self-organizing queue forms at the door due to move_towards_exit_door and collision avoidance, enabling near‐capacity flow (~2–3 exits/step) until the population drops below a critical threshold." ... "After step 24 the evacuation rate settles into a slower decline, with 43 persons leaving over 20 steps (~2.15 persons/step)" ... "High‐density queue/clogging at the single door" (queue explicit; 2.15 and 2–3 agents/step inside the 2.0–2.4 range) |
| 2 | 1 | "Between steps 0–23, evacuation proceeds near the door’s capacity, removing 2–3 agents per step and yielding a roughly linear decline from 100 to 49 persons." (near-linear capacity-limited flow identified for the first half only; the report then overlays a mid-run "pronounced throughput surge" and a "phase transition" to a "sparse, stochastic exit regime", which the near-linear data does not show) |
| 3 | 2 | "The simulation completes in 46 steps with all 100 persons removed." ... "the final agent leaves at step 44" (measured 45 steps = 13.5 s; both 44 and 46 lie within the data-supported 44–47 steps; no conversion to seconds given; no 110 s claim) |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'A pronounced throughput surge follows from steps 14 to 24, where 33 persons exit in 10 steps (~3.3 persons/step), marking the peak removal rate.' (repeated as "Peak evacuation throughput (~3.3 persons/step) occurs between steps 14–24" and "Mid-run throughput surge emerges from synchronized agent arrivals at the door") - the data show a monotone near-linear discharge at ~2.2 agents/step (data-supported 2.0–2.4) with no acceleration; a sustained 3.3/step window is outside the range and the report's own figures (50 evacuated by step 21, 56 by step 24) do not support it

Hallucinated phenomena (verbatim + why unsupported): count = 2
- 'Phase transition from dense, capacity-limited flow to sparse, stochastic exit regime around 50% evacuation.' (also "the system transitions to a sparse regime ... fluctuation-dominated tail") - the data are near-linear to the end (98 at step 43, 99 at 44, 100 at 45); the report's own post-step-24 rate (~2.15/step) equals the steady ~2.2/step rate, so no regime change is supported
- 'Peak instantaneous flow occurs at step 29 (4 exits), slightly exceeding the nominal capacity of three, due to sequential update ordering.' (also "Sequential update ordering → occasional burst of 4 exits in a single step") - a mechanism asserted as fact with no support in the exported data; nothing in the data speaks to update ordering or a nominal capacity of three

Actionable recommendations: y - quote: "Increase door width (e.g. to 4+ cells) and re‐run to quantify reductions in queue length and evacuation tail time."

Language/format notes: English as requested; all three sections (dynamics, spatial, emergence) plus executive summary, conclusions and recommendations present. Internal wobbles (notes, not errors): completion given as "46 steps" in four places but "final agent exits at step 44" in the emergence section; 50% evacuated by step 21 stated as both "count 50" and "population drops from 100 to 54"; "no one exits at step 43" vs. the data's 98@43/99@44/100@45 is a one-step shift within the 44–47 range. PDF not part of the blind set, not checked.
