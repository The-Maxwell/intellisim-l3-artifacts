# EXP-H scoring - report 9

Meta: sim_id=6a636be64ccaa6983aafedd5 analysis_id=6a63a43f4ccaa6983ac9832b lang=en llm_target=o4-mini generated=2026-07-24T17:43:27.088Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-9.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Analyze this simulation's results. Context: An infectious disease spreads in a population of moving agents. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.", "language": "en", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Single-peaked epidemic: infected count grew from 2 to 22 by step 60 then declined to zero by step 227." ... "produced a classic single‐peaked epidemic: after a latent phase lasting until step 12, infections rose exponentially to 22 active cases (11% of the population) at step 60 before recoveries overtook transmission and drove cases to zero by step 227" |
| 2 | 2 | "In total, 74 agents recovered (37% infection penetration) and 126 remained susceptible, demonstrating incomplete saturation and a self-limiting outbreak" ... "Spatial mixing limited sustained chains of transmission, capping the epidemic at ~36% of the population." |
| 3 | 1 | "the susceptible pool shrinks from 198 to 165 by step 60" ... "As susceptibles plateau around 126 after step 219" ... "High recovered fraction (0→74) and residual susceptibles (~126 after step 219)" |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'By Step 113 and Step 227, snapshots show only susceptible agents—infected and recovered individuals have either moved or changed state' - R(t) is monotone non-decreasing (SIR invariant) and final_recovered = 74, so 74 recovered agents are present at step 227 and recovered agents never change state; the population at step 227 is not "only susceptible".

Hallucinated phenomena (verbatim + why unsupported): count = 2
- 'Multi-wave outbreak dynamics: after a single major peak, spatial mixing generated smaller subsequent waves without external reintroduction.' - the data shows a single hump (peak I = 22 at step 60, then extinction); the reported tail fluctuations of 1–9 infected are not a multi-wave pattern, and the report itself elsewhere calls the outbreak "single-peaked" (also asserted as "Minor secondary waves (1–9 infected between steps 141–177)" and "Damped oscillations of infection counts").
- 'Phase transition in transmission dynamics as susceptible density falls below a critical threshold.' / 'Self-limiting outbreak driven by depletion of susceptible agents.' - at the peak 165/200 = 82.5% of agents were still susceptible and 126/200 = 63% were never infected; final size 0.37 is far below the well-mixed bound 0.822, which the data attributes to spatial suppression, not to susceptible depletion below a herd-immunity threshold.

Actionable recommendations: y - quote: "Vary agent mobility speed or movement rules to assess how reduced or increased mixing alters cluster formation and outbreak magnitude."

Language/format notes: English as requested. Phenomenon 3 scored 1: S decreasing and R increasing are described at several time points but the monotonicity invariant is never stated explicitly, and the snapshot sentence counted above suggests recovered agents "changed state". Phenomenon 2: the report gives the measured final size (37%) and attributes the cap to spatial mixing, but never contrasts it with a well-mixed/analytic bound. Internal wobbles not counted as errors: "Final tally: 74 recovered, 126 susceptible; peak infection reached only 37% of the population." mislabels the cumulative infected share as "peak infection" (peak is correctly given as 22 = 11% elsewhere in the same section); "eventually to zero removals by step 227" is garbled; "confirming full recovery of all agents" means all infected agents; final size given as "37%" (74/200) in most places and "~36%" (72 new infections/200) once, both within 0.30-0.40. Intermediate values (I = 6 at step 12, R = 30 at step 75, infection_timer 1.26 at step 66, movement totals) are not verifiable from the sheet and were treated as unknown.
