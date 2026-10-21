# EXP-H scoring - report 8

Meta: sim_id=6a634dbf4ccaa6983a640151 analysis_id=6a63a4084ccaa6983ac98328 lang=en llm_target=o4-mini generated=2026-07-24T17:42:32.071Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-8.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: Fire spreads through a forest of trees. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Ash count rises monotonically from 0 to 2 546 by step 84, converting 97.8% of trees to ash." ... "By termination (step 86), 2547 of 2605 trees burned, leaving 58 survivors (2.2% of peak 2521)" ... "Ultimately, the fire consumes ~98% of trees" |
| 2 | 2 | "Ultimately, the fire consumes ~98% of trees, demonstrating near‐complete percolation through the forest with only isolated refugia remaining at the end." ... "Fire front advanced in a planar wave from left to right" ... "pockets of unburned trees persisted, particularly in the upper‐right quadrant (x>50, y>50)" ... "complete penetration through dense zones" |
| 3 | 2 | "The fire spread as a single bell‐shaped wavefront: initial dip to 21 burning trees by step 4, mid‐simulation peak of 48 at step 25, and decline to zero by step 85." ... "Burning trees steadily drop from 48 to 1 by step 84" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 1
- 'Initial tree agents spanned approximately x=3 to x=54 with no significant voids' - the data records ash in the rightmost column x=60 (percolated = True), so trees existed (and burned) at x=60; the tree population was not confined to x<=54.

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Vary initial tree density around the percolation threshold to identify critical densities for sustained wave propagation and natural firebreak formation."

Language/format notes: English as requested; clean structure. Percolation is named explicitly ("near‐complete percolation through the forest") though the phrasing conflates percolation with burned fraction; burn-through to the far edge is stated indirectly (front "from left to right", survivors only as isolated pockets at x>50). Internal wobbles not counted as errors: termination given as "step 85" in most places but "step 86" in the spatial section; "No oscillations or reignitions occur" vs "The burning counts exhibit damped oscillations beyond step 25, with minor secondary peaks (e.g., ~39 at step 53)"; "Continued left‐edge ignitions (84 executions) → sustained front pressure" implies ongoing ignitions while the dynamics section places all 84 ignitions at step 0; ash total 2 546 (step 84) vs 2 547 (step 85). Claims about "synchronous updates", specific survivor coordinates (x=3 reference_id 47, x=6 id 11), the upper-right survivor quadrant and the ~39 secondary peak are not verifiable from the sheet and were treated as unknown.
