# EXP-H scoring - report 11

Meta: sim_id=6a5bbcaac44d001ef3ac207c analysis_id=6a63a4cd4ccaa6983ac98331 lang=en llm_target=o3 generated=2026-07-24T17:45:49.583Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-1.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a single room through one door. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o3"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "a self-organised queue at the doorway sustained ~3.1 exits per step ... local doorway density peaked ~0.67 person/cell (~20 agents in 30 cells)" |
| 2 | 2 | "capacity-limited phase (steps 10-34) ... quasi-steady discharge regime" |
| 3 | 2 | "evacuated all 100 occupants ... in 46 simulation steps (last agent leaves around step 45)" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Run width-sensitivity experiments (e.g., 1- to 5-cell doors) to quantify how clearance time scales with bottleneck capacity"

Language/format notes: STRETCH (o3), en, same sim as report 1. Correctly infers 0.4 m/cell (25.7 cells = 10.3 m) and door width 1.2 m (3 cells). Richer quantitative spatial characterisation than the o4-mini pilot.
