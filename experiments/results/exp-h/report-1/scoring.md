# EXP-H scoring - report 1

Meta: sim_id=6a5bbcaac44d001ef3ac207c analysis_id=6a63a2404ccaa6983ac98313 lang=en llm_target=o4-mini generated=2026-07-24T17:34:56.228Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-1.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: People are evacuating a single room through one door. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "A self-organizing, single-file queue emerges from local collision avoidance and the single-door geometry, sustaining near-capacity flow" |
| 2 | 2 | "removing 2-3 agents per step and yielding a roughly linear decline from 100 to 49 persons" |
| 3 | 2 | "The simulation completes in 46 steps with all 100 persons removed" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Increase door width (e.g. to 4+ cells) and re-run to quantify reductions in queue length and evacuation tail time"

Language/format notes: en; all sections + PDF + conclusions present, 0 warnings. Minor internal wobble (50 vs 54 remaining near step 21) — not a data contradiction.
