# EXP-H scoring - report 3

Meta: sim_id=6a5bc249c44d001ef3af684d analysis_id=6a63a3254ccaa6983ac98319 lang=en llm_target=o4-mini generated=2026-07-24T17:38:45.776Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-3.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: 150 people move out of a building section consisting of two rooms connected by a corridor. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 1 | "The narrow door at both ingress and egress points generates pronounced upstream congestion: while the corridor and downstream Room 2 quickly clear, occupancy builds immediately upstream of the door until only three agents occupy that zone." ... "Spatial segregation into high‐density upstream cluster and empty downstream region." (upstream-vs-downstream asymmetry identified qualitatively only; no occupancy figures (data: 37.2 vs 12.7, ratio 2.9) and the report elsewhere places the persistent queue "at the final exit") |
| 2 | 2 | "1.2 m door width → limited throughput → accumulation of agents upstream." ... "Long‐lived residual group of 3 agents, self‐trapped by bottleneck despite continuous movement rules." ... "Overall, door width is the critical parameter controlling throughput and residual congestion patterns." (limiting element = narrow door with sustained upstream accumulation; exactly 3 wedged agents, matching the data) |
| 3 | 2 | "147 of 150 agents evacuated by step 100; 3 remain in a persistent congested cluster." ... "Slow‐exit tail: 3 persons remain from step 100 to 599." (data: population ≤3 at step 98, final 3 stragglers to max_steps 599; within the 90–100-step range) |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'The effective corridor width (2 m) supports rapid clearance, whereas the 1.2 m doors limit throughput, leading to arch‐like queuing patterns at the final exit.' (also "By step 299, only 3 of 150 agents remain queued at the exit", "Self‐organized single‐file queue at the exit bottleneck", "unable to pass the narrow exit") - the data localise the queue at the Room-1 door, not the Room-2 exit (peak zone-1 occupancy 37.2 vs zone-2 12.7, non-overlapping across reps), and the 3 stragglers wedge at Room-1 door/corridor pinch points; the report's own coordinates for them (x=24–27, y=13) lie in Room 1 (x≈2–26), contradicting its "at the exit" label

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'This persistent residual group reflects a metastable congested state: although capacity allows periodic exits (147 in 70 steps, ≈2.1 agents/step), local clogs at the door intermittently halt flow.' (also "Metastable congestion: residual group of 3 agents trapped indefinitely") - the data show exactly 3 agents wedged at fixed geometric pinch points in every rep, with positions unchanged to step 599 (as the report itself states); there is no support for intermittent clog/release dynamics or a metastable state after ~step 98

Actionable recommendations: y - quote: "Perform a parameter sweep on door widths (e.g., 1.2 m, 1.5 m, 2 m) to quantify throughput gains and eliminate long‐tail congestion."

Language/format notes: English as requested; all three sections plus executive summary, conclusions and recommendations present. The report reasons from a ~10-step sampled series (steps_covered count 61 over 0–599): "No agents exit before step 30" is inconsistent with its own "At step 30 the first exits occur, reducing the population to 141" (9 exits already between the step-20 and step-30 samples) - internal wobble, not checkable against the sheet. Throughput figures (~2.8 and ≈2.1 agents/step) and the 4,889/4,949 rule counts are not in the sheet and were not scored. Section titles leak the internal run name "exp-B12-bottleneck". PDF not part of the blind set, not checked.
