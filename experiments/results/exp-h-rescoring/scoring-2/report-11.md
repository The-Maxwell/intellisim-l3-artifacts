# EXP-H scoring - report 11

Meta: sim_id=6a5bbcaac44d001ef3ac207c analysis_id=6a63a4cd4ccaa6983ac98331 lang=en llm_target=o3 generated=2026-07-24T17:45:49.583Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-1.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a single room through one door. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o3"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Full clearance achieved in 46 steps; average throughput ≈2.2 persons/step (100 ÷ 46)." ... "36 of the 50 remaining agents (72 %) lay inside the tight rectangle x = 34–39, y = 16–21. Within this 6 × 5 (30-cell) patch, up to 20 agents were present, giving local densities near 0.67 person / cell and forming a visible queue." (sheet: ~2.2 agents/step, range 2.0-2.4; door queue - within range) |
| 2 | 1 | "Outflow was not uniform." ... "This plateau indicates that the outflow is governed by door capacity rather than by how many agents still remain in the room." ... "Clearing interior space → reduced hindrance for late agents → maintained near-constant exit rate." (flow-limited, capacity-governed egress identified, but the report explicitly characterises the discharge as non-uniform with a lag / 3.1-per-step plateau / decay structure and never as near-linear; plateau rate outside the 2.0-2.4 range - partial/imprecise) |
| 3 | 2 | "Run exp-C-rep3 evacuated all 100 occupants from a 38×38-cell room through a single 3-cell door in 46 simulation steps" ... "The last agent leaves around step 45, giving a total clearing time of 45 steps." (sheet: 45 steps = 13.5 s; ensemble 44-47 steps - within range; no ~110 s claim) |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'Capacity-limited plateau: 65 exits between steps 10–30, averaging 3.1 agents/step, with a peak of 5 at step 24.' / 'a self-organised queue at the doorway sustained ~3.1 exits per step' - the data-supported steady door throughput is ~2.2 agents/step (range 2.0-2.4); a sustained 3.1/step over half the run contradicts the near-linear ~2.2/step discharge.

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Phase transition from capacity-limited to demand-limited regime once occupancy drops below ~20, causing a sharp decline in exit rate.' (also 'Throughput collapses to ≈1.1 agents/step after the remaining population falls below 20 (steps 34–45).') - the data shows a monotone near-linear discharge from step ~2 to step 45 with no early plateau and no acceleration; an asserted two-regime structure with a sharp late collapse is not supported by the data (the report's own numbers are also inconsistent: 20 agents over 11 steps is ~1.8/step, not 1.1).

Actionable recommendations: y - quote: "Run width-sensitivity experiments (e.g., 1- to 5-cell doors) to quantify how clearance time scales with bottleneck capacity."

Language/format notes: English, fluent; all three sections, executive summary, 6 conclusions and 6 recommendations present. Internal wobble (not counted as error): total clearance given as "46 steps" in dynamics/spatial/summary/conclusions but "45 steps" in the emergence section - both inside the 44-47 range. Egress time is reported only in steps, never converted to seconds. The "Initial six-step lag: only 4 exits by step 6" claim runs against the sheet's "no early plateau" characterisation but the sheet gives no measured value at step 6, so it is left as a note rather than counted.
