# KEY-NUMBERS — single source of truth for every number the Level-3 article may print

> Built 2026-09-08 (Task 2). Every value below was re-derived from files in THIS repo by `harness/exph_rescore.py` (→ `results/exp-h-rescoring/rubric.csv`) and `harness/exph_cost.py` (→ `results/_billing/exph-cost.md`), or read directly from the cited file. `article/03-key-numbers.md` points here; the manuscript may not print a number that is absent from this file. Pass-2 (blind re-scoring) and κ rows are added by Task 3. Corrections to the sibling's REPORT.md are in §7 — the article uses the corrected values.

## 1. Design of the evaluation (EXP-H)

| Item | Value | Provenance |
|---|---|---|
| Experiment / date | EXP-H, 2026-07-24 (all 12 analyses generated 17:34–17:46 UTC) | `results/exp-h/manifest.json` (`date`, `generated`) |
| Protocol | pre-registered; §1 design, §2 rubric, §3 criteria | `experiments/36-analysis-eval-protocol.md` |
| Blinding gate | 10 ground-truth sheets + manifest committed at sibling commit `23ff33d` BEFORE the first analysis | `results/exp-h/REPORT.md` (bullet «Blinding gate»); sheets: `results/exp-h/ground-truth/sheet-1..10.md` («Pre-filled: 2026-07-24 (committed BEFORE any analysis generated…)») |
| Reports | 12 = 10 primary (o4-mini) + 2 stretch (o3, re-runs of reports 1–2) | `manifest.json` (`reports[]`), `rubric.csv` (`set`) |
| Languages | 5 en + 5 uk among primary; stretch 1 en + 1 uk | `rubric.csv` (`lang`) |
| Scenario families (5) | room evacuation (EXP-C ×2), RiMEA-type evacuation (EXP-B T12 bottleneck, T4 wide opening), LLM-decision evacuation (EXP-E archetype_decision / individual_decision), wildfire (EXP-F p = 0.50 / 0.70), epidemic SIR (EXP-G med ×2) | `manifest.json` (`source_run`), `rubric.csv` (`family`) |
| Source runs | 10 pre-existing finished simulations; none created or deleted for EXP-H; **their exports + the measured truth files are in `results/source-runs/` (31 files, `MANIFEST.md`, D16)** | `REPORT.md` §Artifact accounting; `results/source-runs/MANIFEST.md` |
| Phenomena per sheet | exactly 3 (30 pre-registered phenomena total for primary; 6 more re-used by the stretch pair) | protocol §2; `ground-truth/sheet-N.md` rows 1–3 |
| Rubric | per phenomenon 2 / 1 / 0; per report: factual errors, hallucinated phenomena, usefulness (y/n, one quote); verbatim quotes mandatory | protocol §2 |
| Max points (primary) | 10 × 3 × 2 = 60 | protocol §3 |
| Criterion 1 | phenomenon points ≥ 70 % (≥ 42 / 60) | protocol §3 |
| Criterion 2 | factual errors mean ≤ 1 per report (≤ 10 total) | protocol §3 |
| Criterion 3 | descriptive, no threshold: hallucinations, usefulness, en/uk, o4-mini/o3 | protocol §3 |
| Second scorer | «Optional inter-rater check: a second scorer scores independently; report Cohen's κ» — foreseen by the protocol | protocol §2 («Scorer») |
| Context messages | neutral, non-leading; banned words listed | protocol §1.2; verbatim bodies in `report-N/request.json`, `manifest.json` |
| LLM calls per report | 4 (3 sections: dynamics, spatial, emergence + 1 synthesis; events/rules section not requested — `selection: {}`) | `manifest.json` (`token_usage.total.calls`), `REPORT.md` bullet 1, `request.json` |
| Engine / harness | core `6bb5bd9` (feature/experiment-enablers); harness `fd2fb3d` | `manifest.json` (`engine_commit`, `harness_commit`) |
| Model deployments | `o4-mini-2025-04-16`, `o3-2025-04-16` | `manifest.json` (`token_usage.by_model[].model_uid`); billing CSV `model` column |
| Табл. 1 ground-truth cells | the per-report measured values printed in the article's «source runs» table (≈ 2,2–2,3 agents/step, 44–45 steps, 147/150 by ≈ step 98, 297/300 by step 83, 98 vs 46 steps, 10,8 ± 6,4 с, 6,76 ± 3,25 с, 494 decisions, 5,9 % / 99,25 % burned, 85 steps, I = 22 @ 60 / 21 @ 51, 74/200 / 66/200, β = 0,18 …) are transcribed from `ground-truth/sheet-N.md` rows 1–3 by `harness/exph_tables.py` → `results/_article/tables/TABLES.md`; the manuscript may shorten the wording but not change a value (Task 9 review, S24). Scenario sizes printed in the same table (100 агентів, один вихід; 150 агентів, дві кімнати та коридор; отвір 4 м, 300 агентів; 200 агентів) come from `sheet-1`, `sheet-3` («wide (4 m / 10-cell) opening» in `sheet-4`), `sheet-9` (recorded 2026-10-18, v8) | `results/exp-h/ground-truth/sheet-1..10.md`; `results/source-runs/MANIFEST.md` |
| Табл. 2 pass-III cells (v6, D31) | per-report pass-3 scores of the primary series printed next to passes I–II: Явище 1–3 Σ 14 / 13 / 13, points **40**, errors **8**, hallucinated phenomena **11** (per report: 1/1/2 4 1 1 · 2/1/1 4 1 2 · 1/2/2 5 1 1 · 1/1/0 2 2 1 · 1/2/1 4 1 2 · 0/0/2 2 0 1 · 2/2/1 5 0 1 · 2/1/2 5 1 0 · 2/2/1 5 1 1 · 2/1/1 4 0 1); reports 11–12 were not scored in pass 3 → «–»; the all-«так» recommendation column moved to the text | `results/exp-h2/rubric3.csv` (arm `orig`) via `harness/exph_tables.py` `read_rubric3_orig()`, asserted against `OUTCOMES.json` |

## 2. Primary results (pass 1, original scoring 2026-07-24)

| Item | Value | Provenance |
|---|---|---|
| Phenomenon points | **54 / 60 = 90 %** (PASS; threshold 42) | `rubric.csv` pass 1 Σ points reports 1–10; `REPORT.md` verdict |
| Mean per report | 5.4 / 6 | 54 / 10 |
| Perfect reports (6/6) | **6 of 10** (reports 1, 3, 7, 8, 9, 10) — NOT 7, see §7 | `rubric.csv` |
| Reports ≥ 5/6 | 9 of 10 | `rubric.csv` |
| Lowest report | report 4 (EXP-B T4, uk): 3 / 6 (2 / 1 / 0) | `report-4/scoring.md` |
| Per-phenomenon column sums | P1 = 20 / 20; P2 = 18 / 20; P3 = 16 / 20 | `rubric.csv` (p1, p2, p3 over reports 1–10) |
| Score distribution (30 items) | 2 → 25 items; 1 → 4 items; 0 → 1 item | `rubric.csv` |
| Factual errors | **7 total, mean 0.7 / report** (PASS; threshold ≤ 1); per report: 0,1,0,2,1,1,2,0,0,0; max 2 | `rubric.csv` (`errors`); transcripts in `report-N/scoring.md` |
| Reports with 0 errors | 5 of 10 (1, 3, 8, 9, 10) | `rubric.csv` |
| Hallucinated phenomena | **1** (report 4: density–flow «optimum») | `report-4/scoring.md` |
| Usefulness | 10 / 10 reports contain actionable recommendations (binary, self-assessed, quote each) | `rubric.csv` (`useful`) |
| Completion | 12 / 12 `completed`; 0 warnings; Cyrillic OK in JSON + PDF | `manifest.json` (`status`); `REPORT.md` bullet 3 |

## 3. Contrasts (descriptive only — n = 5 / 5 and 2 / 2; no tests)

| Split | Reports | Points | Mean / 6 | Errors | Provenance |
|---|---|---|---|---|---|
| en | 1, 3, 5, 8, 9 | 29 / 30 (96.7 %) | 5.8 | 1 | `rubric.csv` |
| uk | 2, 4, 6, 7, 10 | 25 / 30 (83.3 %) | 5.0 | 6 | `rubric.csv` |
| o4-mini, pair (1, 2) | same two EXP-C runs | 11 / 12 | 5.5 | 1 | `rubric.csv` |
| o3, pair (11, 12) | same two EXP-C runs | 12 / 12 | 6.0 | 0 | `rubric.csv` |

uk gap is driven by report 4 (3/6, 2 errors) and report 7 (2 errors) + the report-2 time-unit slip; `REPORT.md` §en vs uk: «not a Cyrillic-pipeline defect; small n». Without report 4 the uk set is 22 / 24 with 4 errors (derived; state only as such).

## 4. Qualitative findings with exact quotes (for the results section)

| Finding | Value / quote | Provenance |
|---|---|---|
| Exact reproduction of decision-label counts | 441 proceed / 178 follow_crowd / 40 seek_alternate_route / 55 wait_briefly (714 decision events) | `report-5/scoring.md` row 2; `ground-truth/sheet-5.md` |
| Exact epidemic peak and final size | peak I = 22 at step 60; final size 74 / 200 = 37 % | `report-9/scoring.md` rows 1–2; `sheet-9.md` |
| Super-critical burn | 97.8 % of trees burned (2 547 ash) vs measured 0.9925 for the rep (sweep mean 0.976) | `report-8/scoring.md` row 1; `sheet-8.md` |
| Sub-critical regime | fire dies by ≈ step 10, ≈ 6–8 % burned, no burn-through | `REPORT.md` §highlights; `report-7/scoring.md` |
| Bottleneck localisation | congestion upstream (Room 1), downstream clear; 3-agent straggler cluster; clearance ≈ step 100 | `report-3/scoring.md` |
| Reads data, not prior | EXP-C egress reported as measured 44–46 steps in reports 1, 2, 11, 12 — never the SFPE ≈ 110 s textbook value (engine deviates 6.6× from SFPE, sibling D29) | `sheet-1.md` row 3; `REPORT.md` §highlights |
| Unit-inference contrast | report 2 (o4-mini, uk): «кроці 43 (≈ 57 с)» — wrong absolute time (0.3 s/step ⇒ ≈ 13 s); report 12 (o3, uk): «45-му кроці (≈ 13,5 с) … один крок ≈ 0,30 с», also inferred 0.4 m/cell | `report-2/scoring.md` rows 3 + error 1; `report-12/scoring.md` row 3 + notes |
| Boundary case | report 4: T4 «fundamental diagram» truth is an ensemble property (N = 50/150/300) not observable in one N = 300 run → report over-interpreted a density–flow optimum («максимум ≈ 6,5 агентів/крок при ≈ 160 осіб») and mis-attributed low early flow to congestion; scored 3/6, 2 errors, 1 hallucination | `report-4/scoring.md`; `REPORT.md` §weak |
| Minor slips | report 5: permanent straggler implied where run reached `condition_met`; report 6: «single-cell door» in summary (three-cell elsewhere); report 7: fire-front direction reversed once + 4.4 % vs 138-ash inconsistency | `report-5/6/7/scoring.md` |

## 5. Cost, tokens, latency (definition of record = billing × list price; see `results/_billing/exph-cost.md`, DECISIONS D13)

| Item | Value | Provenance |
|---|---|---|
| **EXP-H cost of record** | **$1.03** (list price; 2026-07-24 billing rows for o4-mini + o3) | `exph-cost.md` §Cost of record |
| Per report (all 12) | **≈ $0.086** | $1.0269 / 12 |
| o4-mini (10 reports) | $0.853 total; ≈ $0.085 / report; 41 billed requests; 132 194 in / 160 920 out tokens | billing CSV row 2026-07-24 `o4-mini-2025-04-16` |
| o3 (2 reports) | $0.173 total; ≈ $0.087 / report; 8 requests; 22 001 in / 16 176 out | billing CSV row 2026-07-24 `o3-2025-04-16` |
| List prices (per 1 M tokens) | o4-mini $1.10 in / $4.40 out; o3 $2.00 in / $8.00 out — verified 2026-09-08 at https://developers.openai.com/api/docs/pricing; **re-verified live 2026-10-18 (same values)** and cited in the manuscript as `[OpenAI-pricing]` (v8, D33; `PRICE_VERIFIED` left at 2026-09-08) | `harness/exph_cost.py` (`PRICE`, `PRICE_VERIFIED`); `article/04-citation-register.md` |
| Engine-based estimate (NOT of record) | $0.585 (o4-mini $0.412 + o3 $0.173) | `results/exp-h/cost.json` |
| Engine tokens, primary | 130 755 prompt / 60 917 completion (35 072 reasoning — NOT 34 072, §7) / 40 calls | `manifest.json`; `exph-cost.md` |
| Engine tokens, stretch | 22 001 prompt / 16 176 completion (9 792 reasoning) / 8 calls | `manifest.json` |
| Engine vs billing | o3 exact (calls, in, out); o4-mini input ×1.011, **output ×2.64** (60 917 engine vs 160 920 billed), 41 vs 40 requests | `exph-cost.md` §reconciliation; `_billing/reconciliation.md` |
| o3 vs o4-mini cost | on billing nearly equal per report ($0.087 vs $0.085); the engine-based «o3 ≈ 1.9×» in REPORT.md is an artefact of the under-count — do not use | `exph-cost.md` |
| Latency per report | 28.8–60.1 s; mean 44.8 s (o4-mini 43.2 s; o3 52.6 s) | `manifest.json` (`latency_ms`) |
| PDF size | 102–155 kB (decimal) | `report-N/analysis.pdf` sizes |
| Cached input | o4-mini 1 280 cached tokens (≈ $0.001 discount, ignored) | billing CSV `input_cached_tokens` |

## 6. Level-3 implementation facts (for the architecture section)

| Item | Value | Provenance (`intellisim.core`, branch dev @ `5072b5f`; EXP-H ran on `6bb5bd9`) |
|---|---|---|
| Entry point | `POST /simulations/:id/analyses` → 202 async; SSE `analysis.progress/completed/failed`; PDF via GridFS bucket `analysis_files` | `modules/simulations/analysis_api.go`, `analysis_service.go` |
| Evidence pack (deterministic) | model summary, run summary, `SimulationStatistics` (totals, spawned/removed, interactions, events, rule executions, ByRule/ByEvent/ByInteraction, movement, per-archetype peak/min), per-archetype population series, per-numeric-property mean series (agents + shapes), thinning by step sampling, spatial snapshots at up to 3 steps (from, midpoint, to), ≤ 50 agents each | `analysis_evidence.go` (L129–216: `thinPopulation`, `thinSeries`, «Spatial snapshots at up to 3 steps», `const maxAgents = 50`) |
| Sections | `dynamics`, `spatial` (spatial models), `emergence` (always), `events` (only when requested) | `analysis_prompt.go` L15–18 |
| Request contract (v6, D31) | `context_message` required, **≤ 4 000 characters**, inserted after the evidence in every call; `language` en/uk; `llm_target` (model with `results_analysis` capability; effort, generation params, timeout optional); optional `selection`: step window (frozen at request), `sampling`, `agent_uids`, `property_uids`, `focus_areas` ⊆ {population, properties, spatial, interactions, events, rules} | `analysis_dtos.go` L10–26, `analysis_service.go` L112–164 |
| Calls per analysis (v6, D31) | planned sections + 1 synthesis = **2–6** (min: emergence + synthesis; max: 5 sections + synthesis); **4 in EXP-H** (dynamics, spatial, emergence + synthesis; all 10 models spatial, no focus areas); «динаміка» ↔ population/properties, «просторова структура» ↔ spatial/interactions | `analysis_pipeline.go` L138–164 (`plannedSections`), `results/exp-h/report-*/report.json` |
| Section JSON contract | `{title, narrative, key_findings, causal_links, emergent_phenomena}`; synthesis `{executive_summary, conclusions, recommendations}` | `analysis_llmcontract.go`, `analysis_prompt.go` |
| Section prompt bounds (v8, 2026-10-18) | narrative **2–5 paragraphs**, **3–7** key findings per section — printed in Крок 4 since v6, added here so the «no number outside KEY-NUMBERS» rule holds literally | `analysis_prompt.go` L76 («narrative: 2–5 paragraphs … key_findings: 3–7 short bullet insights») |
| Reasoning mode | chosen when the model advertises reasoning; invalid JSON → retry while budget remains | `analysis_llm.go` L35–59 |
| Rendering | go-chart line charts + Maroto v2 PDF, embedded DejaVu (Cyrillic) | `analysis_charts.go`, `analysis_pdf.go` |
| Fail-soft | warnings such as `section_failed`, `chart_skipped` instead of aborting | `analysis_pipeline.go`, `analysis_errors.go` |
| Not present (honest scope) | multi-run comparison, statistical tests, anomaly detection, calibration, raw per-agent frames / agent memory in evidence, spatial diagrams in PDF | verifiable against `modules/simulations/analysis_*.go` (design document and internal evidence assessment not included in this archive) |

## 7. Corrections to the sibling `REPORT.md` (article uses the corrected values)

| # | REPORT.md says | Correct value | Evidence |
|---|---|---|---|
| C1 | «7 of 10 primary reports scored a perfect 6/6» | **6 of 10** (1, 3, 7, 8, 9, 10); sum 54, mean 5.4, «nine of ten ≥ 5/6» remain correct | REPORT.md table itself; `rubric.csv`; `exph_rescore.py` assertion `perfect_6 == 6` |
| C2 | o4-mini reasoning tokens 34 072 | **35 072** | `manifest.json` per-report values (which REPORT.md's own table source, `cost.json`, also carries) summed by `exph_cost.py` |
| C3 | EXP-H list cost «≈ $1.4» (Deviations; also `reconciliation.md`) | **$1.03** (o4-mini $0.853 + o3 $0.173) — the «$1.4» sums do not reproduce from the stated tokens and prices | `exph-cost.md` |
| C4 | «o3 cost ~1.9× the o4-mini per-report token spend» | true only for ENGINE tokens; on billing the per-report cost is ≈ equal (0.087 vs 0.085) | `exph-cost.md` |
| C5 | PDF «102–155 KB» | same numbers, decimal kilobytes (bytes / 1000); binary would be 100–151 KiB | file sizes |

## 7b. Corrections found 2026-09-23 (D25, D26) — supersede §5 rows «Engine vs billing», «o3 vs o4-mini cost», «Latency per report» and correction C4

| # | Previously stated | Correct value | Evidence |
|---|---|---|---|
| C6 | «engine under-counts o4-mini output ×2,64» (a systematic reasoning-token gap); C4 «on billing the per-report cost is ≈ equal» | **One timed-out attempt, not a systematic gap.** Billing − engine (2026-07-24, o4-mini) = exactly +1 request, +1 439 input, +100 003 output tokens (**$0.442** at list price); the other 40 calls reconcile to the token (130 755 / 60 917), as o3 does (22 001 / 16 176). Report 2's wall-clock is 105,4 s vs 44,1 s summed call latency (gap 61,3 s = provider timeout 60 s + 1 s retry backoff); all other gaps ≤ 4,2 s. Per report without that attempt: **o4-mini ≈ $0.041** (engine $0.4119 / 10), **o3 ≈ $0.087** → o3 ≈ 2,1× o4-mini. The cost of record **$1.03** is unchanged (it includes the attempt). | `results/exp-h/report-N/report.json` created/finished vs `token_usage.total.latency_ms`; provider `internal/config/config.go:22`, `internal/provider/openai.go:167-182`; core `analysis_llm.go:52-83`; billing CSV |
| C7 | «тривалість від запиту до події завершення» printed as 28,8–60,1 s, mean 44,8 s | those are **summed call latencies**. **Wall-clock (created → finished): 29,2–105,4 s, mean 50,7 s, median 46,6 s (12 reports); 29,2–60,2 s without report 2.** Primary 10: mean 50,2 s | `report.json` `created_at` / `finished_at` |
| C8 | «по чотири виклики моделі на звіт» | 4 **successful** calls per report; report 2 had 5 provider requests (one timed out and was retried) | C6 |
| C9 | «Контекстна підказка була нейтральною …» (implying no leading information reached the model) | the hint was neutral, but the **model name + description (part of M) and the simulation name (part of R) reach every prompt**; report 3's description states sheet-3 P1 («congestion localizes upstream (zone-1 occupancy >> zone-2)»), report 4's description and hint prime flow–density, EXP-F names carry p, EXP-C/E descriptions give 0,4 m/cell and 1,333 m/s (source of o3's 0,30 s/step). Sensitivity: without report 3's P1 + P2 → **50/56 (89,3 %)** pass 1, **43/56 (76,8 %)** pass 2 | D26; `experiments/37-…md` §3 + Appendix A |
| C10 | reproducibility unstated | `o4-mini-2025-04-16` API shutdown **2026-10-23**, `o3-2025-04-16` **2026-12-11** (OpenAI deprecations page, fetched 2026-09-23) | https://developers.openai.com/api/docs/deprecations |

## 7c. Corrections found 2026-09-24 (v5 general review, D30) — supersede the rows named; no score, count or cost changes

| # | Previously stated | Correct statement (print form in v5) | Evidence |
|---|---|---|---|
| C11 | §4 «Reads data, not prior»: «never the SFPE ≈ 110 s textbook value (engine deviates 6.6× from SFPE)»; v1–v4 printed «довідникове значення близько 110 с …, від якого час спорожнення в рушії відрізняється в 6,6 раза» | ×6,6 = engine T_drain 12,97 s vs the pre-registered SFPE band **72,6–98,3 s** for this geometry (band centre / 12,97 = 6,59). The ≈ 110 s canonical worked example is «reference only» and would give ≈ 8× (sheet 2 says «~8x»). v5: «…а не довідникове значення за гідравлічною моделлю [SFPE], від якого час спорожнення в рушії відрізняється в 6,6 раза» (no seconds value) | sibling `DECISIONS.md` D29; `intellisim.experiments/results/exp-c/REPORT.md` L15, L42; `ground-truth/sheet-1.md`, `sheet-2.md` row 3 |
| C12 | §6 / `02-terminology.md`: series «розріджені до ≤ 60 точок»; Рис. 2 «ряди ≤ 60 точок» | `downsampleSeries` / `downsamplePopulation` use the integer stride `len / MaxSeriesPoints` (60): series of ≤ 119 points are not thinned, longer ones keep 60–119 points. v5: «Ряди, довші за 60 точок, розріджуються зі сталим кроком»; Рис. 2 «розріджені ряди» | core `6bb5bd9` `modules/simulations/analysis_evidence.go` L347–380, `analysis_config.go` L15; `report.json` `steps_covered.count` 97 / 86 / 77 / 88 (reports 5, 8, 9, 10) |
| C13 | §6 «Sections»; v1–v4 «Секції «динаміка» і «емерджентність» плануються завжди … «події та правила» – за запитом» | `plannedSections`: **emergence always**; dynamics when the focus includes population/properties (the default); spatial for spatial models (default focus); **events and rules are two separate sections**, each only when explicitly requested and non-empty. v5 prints exactly this; Рис. 2 planning box likewise | core `6bb5bd9` `analysis_pipeline.go` L137–163 |
| C14 | §7b C6 / D25: the timed-out attempt was retried by the provider (60 s timeout + 1 s backoff); v4 «яку мікросервіс-адаптер перервав за 60-секундним тайм-аутом і повторив» | The adapter timed out at 60 s, but the **successful repeat came from the Level-3 pipeline** (`callJSON`, `MaxRetries` = 1): the provider starts its latency clock before its own retry loop, so an adapter-internal retry would sit inside the reported latency, whereas report 2's 61,3 s gap lies outside it. Likely mechanism (not verified live): the adapter's own retry re-sends an already consumed request body. v5: «…спробу виклику у звіті 2, яку після 60-секундного тайм-ауту мікросервісу-адаптера було повторено». $0.44 / $1.03 unchanged | provider `internal/provider/openai.go` L64–108 (clock), L150–186 (`buildHTTPRequest` body, `doRequest` loop); core `analysis_llm.go` L36–59, `analysis_config.go` L16–17 |
| C15 | §9 «Pass-2 errors + hallucinations re-checked: 25 of 25 (14 + 11)»; `ANALYSIS.md` «map one-to-one onto its 14 + 11»; print form «10 числово неточні» / body «10 – правдивими якісно за неточного числа» | The 25 checked **statements** are not one-to-one with pass 2's 25 counted **items**: the undisputed «≈ 57 с» error (report 2) has no row (2d: «not re-checked»), and report 4's single attribution error is split into rows 4b + 4c. The tally 6 / 10 / 8 / 1 stands **over statements**. PARTLY covers «number off» (11b, 2b, 12a, 10a) and «number right, interpretation wrong» (1b, 3b, 4b, 4c, 4e, 6a). v5 print forms: abstract «З 25 спірних тверджень … 10 визнано частково правдивими»; body «З 25 перевірених тверджень, на яких ґрунтувалися помилки й галюциновані явища другого проходу …, а 10 – частково правдивими (неточне число або тлумачення)»; EN «Of the 25 disputed statements … 10 partly true». The item-level alternative (6 / 9 / 9 / 1, counting 57 s as refuted) is not printed. Lower bound 10 and «5 спростованих тверджень … пропустив» unaffected | `DATA-CHECK.md` rule L7 and per-claim rows; `scoring-2/report-2.md` L16, `scoring-2/report-4.md` L16 |

Open (no print change): the «11 з 14 помилок і 9 з 11 галюцинованих явищ» split of §8 is a Task 3 classification (`ANALYSIS.md` §2) without an item list — add the list before submission (R1 S22, 2026-09-24).

## 8. Second (blind) scoring pass and inter-pass agreement (Task 3, 2026-09-08; D17)

Source: `results/exp-h-rescoring/{rubric.csv,kappa.md,ANALYSIS.md,PROTOCOL.md}`, sheets `scoring-2/report-N.md`. Both passes LLM-assisted (D12). Nothing adjudicated.

| Item | Pass 1 | Pass 2 | Provenance |
|---|---|---|---|
| Phenomenon points (1–10) | 54/60 = 90.0 % | **46/60 = 76.7 %** | `kappa.md` |
| Criterion 1 (≥ 42) | PASS | PASS | `kappa.md` |
| Perfect reports / ≥ 5 | 6 / 9 | 1 / 7 | `kappa.md` |
| Factual errors (1–10), mean | 7 (0.7) | **11 (1.1)** | `kappa.md` |
| Criterion 2 (≤ 10 total, mean ≤ 1) | PASS | **FAIL** | `kappa.md` |
| Hallucinated phenomena (1–10) | 1 | 9 | `kappa.md` |
| Usefulness | 10/10 | 10/10 | `kappa.md` |
| en / uk points | 29 / 25 | 26 / 20 | `kappa.md` |
| en / uk errors | 1 / 6 | 5 / 6 | `kappa.md` |
| o4-mini pair (1, 2) points / errors | 11 / 1 | 9 / 3 | `kappa.md` |
| o3 pair (11, 12) points / errors | 12 / 0 | 11 / 3 | `kappa.md` |
| All-12 totals: points / errors / hallucinations | 66 / 7 / 1 | 57 / 14 / 11 | `kappa.md` |
| Cohen's κ, 30 primary items | **0.437** (linear-weighted 0.487); exact agreement 73.3 % | | `kappa.md` |
| Cohen's κ, all 36 items | 0.422 (weighted 0.472); exact agreement 75.0 % | | `kappa.md` |
| Confusion (36 items, p1 rows × p2 cols) | 0→0: 1; 1→1: 4; 2→2: 22; **2→1: 9**; all other cells 0 | | `kappa.md` |
| Disagreement direction | all 9 phenomenon disagreements are 2 → 1 (pass 2 stricter); no reversals | | `kappa.md` |
| Error-count exact match | 2/12 reports; mean abs. diff. 0.92 | | `kappa.md` |
| Hallucination-count exact match | 4/12; mean abs. diff. 0.83 | | `kappa.md` |
| Usefulness match | 12/12 | | `kappa.md` |
| Nature of disagreement | within-run regime claims (surge / phase transition / secondary waves / plateau ≈ 3 per step) counted as errors or hallucinations by pass 2 (11 of 14 errors, 9 of 11 hallucinations), as notes by pass 1; 5 genuine catches by pass 2 (reports 3, 4, 8, 9, 10) | | `ANALYSIS.md` §2 |
| Information asymmetry | pass 1 had the step series; pass 2 only the sheet → part of the gap is not rater disagreement | | `ANALYSIS.md` §Asymmetry |
| Robust in both passes | report 4 weakest (3 → 2 of 6); EXP-C egress reported as measured in all four reports; exact reproductions scored 2/2; o3 unit inference (report 12 P3 = 2) | | `ANALYSIS.md` §2.5 |
| Article-ready range statements | «77–90 % of pre-registered phenomena recovered (two passes)»; «0.7–1.1 factual errors per report»; «κ ≈ 0.44, moderate» | | `ANALYSIS.md` §What the article should say |

## 9. Objective data-check of the disputed statements (Task 3b, 2026-09-09; D18)

Source: `harness/exph_datacheck.py` → `results/exp-h-rescoring/DATA-CHECK.md`, `data-check.csv`; inputs `results/source-runs/**`. Rater-free; no score changed. Thresholds: regime change ≥ 30 % rate difference; SIR rebound ≥ 3× trough and ≥ 5 infected.

| Item | Value | Provenance |
|---|---|---|
| Statements checked | 50 (plus 2 run-level context rows) | `DATA-CHECK.md` §Tally |
| Verdicts, all | TRUE 19 · PARTLY 21 · FALSE 9 · UNCHECK 1 | `DATA-CHECK.md` §Tally |
| Pass-2 errors + hallucinations re-checked | 25 of 25 (14 + 11) | `DATA-CHECK.md` |
| … of which TRUE (pass 2 wrong) | **6** (1a, 11a, 2a, 12b, 12c, 9a — EXP-C rate windows, plateau, late slowdown; SIR rebound) | `DATA-CHECK.md` |
| … PARTLY (number off, statement holds or vice versa) | 10 | `DATA-CHECK.md` |
| … FALSE (pass 2 right) | **8** (1c, 3a, 4a, 5a, 7a, 8a, 9c, 10b) | `DATA-CHECK.md` |
| … UNCHECK | 1 (9e herd-immunity mechanism) | `DATA-CHECK.md` |
| Pass-1 errors/hallucinations re-checked | 5 checkable: FALSE 2 (5a, 7a), PARTLY 3 (4b, 4c, 7c); «57 с» (report 2) undisputed; «одноклітинний отвір» (report 6) not checkable from the export | `DATA-CHECK.md` |
| Pass-2 catches that pass 1 missed, confirmed FALSE | 5 (3a, 4a, 8a, 9c, 10b) + 1c | `DATA-CHECK.md` |
| EXP-C rep3 profile | first exit step 2, 100 out at step 45; 2.22/step overall; R² 0.987; window rates 1.62 (1–13) / 3.30 (15–24) / 2.10 (25–45); max 5 exits at step 24 | row `1/11` |
| EXP-C rep18 profile | first exit step 4, 100 out at step 44; 2.27/step; R² 0.991; 3.09/step (9–30); 3.07 → 1.50 after step 34 (ratio 0.49); max 5 at steps 11, 25 | row `2/12` |
| Report 4 series | 297 out at step 83 (3.58/step); best 5-step window 7.2/step at 172 remaining; band rates 4.21 / 6.80 / 2.14 per step (> 200 / 200–100 / < 100 remaining); 3 stragglers static in the opening (x 39–42) to step 899 | rows `4a`–`4e` |
| SIR rebounds | rep1: 1 (step 138) → 9 (step 177), 9×; rep8: 3 (step 84) → 10 (step 104), 3.3× | rows `9a`, `10a` |
| Data-contradicted statements, lower bound | 9 FALSE + 1 undisputed (57 с) = **10 over 10 primary reports**; 21 PARTLY decide criterion 2 | `ANALYSIS.md` §Task 3b |
| Article-ready statements | «Перерахунок спірних тверджень за експортованими рядами: близько чверті „помилок“ другого проходу — правдиві описи внутрішньопрогонової динаміки з вищою часовою роздільністю, близько третини — підтверджені помилки, решта — числово неточні»; «рубрикатор має фіксувати часову роздільність діапазону» | `ANALYSIS.md` §Task 3b |

Printing rule: the data-check is reported as a diagnostic of the rubric, never as a third score; do not print a per-report «corrected» error count.

## 10. EXP-H2 — regeneration (F2) and model-only ablation (A), 2026-09-23 (D24; `results/exp-h2/REPORT.md`, `OUTCOMES.json`)

| Item | Value (print form) | Provenance |
|---|---|---|
| Design | 10 F2 (same requests, unchanged engine) + 10 A (no run data: model summary + run name/limit/seed/status + same hint; standalone replica of the engine prompts, outside the metasystem); o4-mini (`o4-mini-2025-04-16`), 23.09.2026; pre-registered commit `21a1e2d` | protocol 37; `manifest.json` |
| Pass 3 (blind, sheet-only, one mixed session, 30 reports) | original **40/60 (67 %)**, F2 **44/60 (73 %)**, A **14/60 (23 %)** | `OUTCOMES.json` `arms` |
| Errors / hallucinations, pass 3 | original 8 / 11; F2 5 / 8; A 11 / 14 (per report 0,8 / 0,5 / 1,1 errors) | `OUTCOMES.json` |
| Primary outcome | original > A in **10 of 10** runs, Σ 26 points, sign test **p ≈ 0,001** (1/1024) | `OUTCOMES.json` `primary` |
| A by item class | data-dependent items **0 of 20** points; metadata/prior items 8 of 22; prior-qualitative items 6 of 18; one 2-point item (burn-through at p = 0,70) | `OUTCOMES.json` `A_by_class` |
| Three scorings of the original reports | 54 / 46 / 40 of 60 → **67–90 %**; criterion D ≥ 0,7 met in 2 of 3 scorings; F2 meets it under pass 3 (73 %) | rubric.csv, `rubric3.csv` |
| Agreement, 30 items | pass 2 – pass 3 (both blind) **κ = 0,65** (80 % exact); pass 1 – pass 3 κ = 0,24 (57 %); pass 1 – pass 2 κ = 0,44 (73 %, §8); original – F2 in pass 3 κ = 0,31 (60 % exact), per-run difference −2…+2 points | `OUTCOMES.json`; `kappa.md` |
| Recommendations present | 10/10 in every arm, including A (binary criterion does not discriminate) | `rubric3.csv` |
| Wall-clock per report | A 32–75 s (mean 46,6 s); F2 47–141 s (mean 70,2 s), 47–78 s without F2-10 | `manifest.json` |
| Failed attempts | A 0 of 40; F2 1 inferred (F2-10: 140,7 s vs 78,4 s latency, gap 62,3 s ≈ the 60 s timeout + retry — the D25 timing pattern); **not billed** (no extra request in the export) | `manifest.json`; `COST.md` |
| Cost of record (billing × list price) | **$0.646 → «0,65 дол. США»** (A $0.229, F2 $0.417 → F2 «близько 0,04 дол.» per report); billing = recorded usage to the token (80 requests / 168 039 in / 104 806 out); cached 4 224 tokens (≈ $0.003, ignored) | `COST.md`; billing CSV 2026-09-23 row `o4-mini-2025-04-16` |
| Joint pre-registered acceptance (both criteria) | held **only in pass 1** (D ≥ 0,7: passes 1, 2; ē ≤ 1: passes 1, 3) — printed in v4 Results, abstract, Висновки (S1/R1) | rubric.csv, `rubric3.csv`, `OUTCOMES.json` `criterion*_pass` |
| Count agreement, 10 primary reports, exact match (errors / hallucinations / items) | pass 1–2: 2/10, 4/10, 22/30; pass 1–3: 4/10, 2/10, 17/30; pass 2–3: 7/10, 5/10, 24/30 — both count types agree worse than items in every pair → v4 novelty «найгірше відтворюються підрахунки фактичних помилок і галюцинованих явищ» (S4) | recomputed 2026-09-23 from rubric.csv + rubric3.csv |
| F2 vs A per run | F2 > A in 9 of 10 runs (tie in run 2: 2 = 2) → only «звіти основної серії» win 10/10 (S2) | `OUTCOMES.json` `per_run` |
| Level-3 code identity | `modules/simulations/analysis_*.go` identical between core `6bb5bd9` (EXP-H) and `5072b5f` (EXP-H2); only collision/congestion/runtime/step files differ → «з тим самим кодом рівня 3» | `git diff --stat 6bb5bd9 5072b5f -- modules/simulations/` |
| Duration print form (Практичне значення) | «приблизно за хвилину» (July o4-mini/o3 wall-clock mean 50,7 s incl. report 2; F2 mean 70,2 s) — replaces «≈ 45 с» / «≈ 50 с» (S6) | C7; `manifest.json` |
| Never print | a «corrected» EXP-H score; pass-3 as a replacement for passes 1–2; any p-value other than the pre-registered sign test; «A proves the evidence pack causes …» beyond the same-scorer comparison | protocol 37 §5–§6 |

## 11. Post-hoc pooled statistics over the three scorings (v7, D32; `harness/exph_pooled.py` → `results/_article/POOLED.md`)

> **Not pre-registered.** Pass 1 is the scoring of protocol 36 (the primary, pre-registered verdict: both criteria met); passes 2 and 3 are blind robustness checks (D17, D27). The values below are descriptive and are printed only with «апостеріорно» / «post hoc» in the same sentence; they never replace the pre-registered verdicts of §2, §8, §10, and the joint-acceptance statement («обох критеріїв разом дотримано лише в першому проході») stays.

| Item | Value (print form) | Provenance |
|---|---|---|
| Primary / robustness framing (author, 2026-10-12) | pass 1 = «передреєстрований (основний) прохід» — both criteria met (54/60 = 90 %, 0,7 помилки); passes 2–3 = «сліпі перевірки стійкості» — each met one criterion (pass 2: D 77 % ✓, 1,1 помилки ✗; pass 3: D 67 % ✗, 0,8 помилки ✓); the range 67–90 % stays | `rubric.csv`, `rubric3.csv`; D27 (ii) superseded for the wording only |
| Pooled D over passes I–III | **140/180 = 77,8 %** (points 54 + 46 + 40 of 3 × 60) | `POOLED.md` §2 |
| Pooled mean errors per report-scoring | **26/30 = 0,87 помилки на звіт** (7 + 11 + 8 over 30 report-scorings) | `POOLED.md` §2 |
| Pooled hallucinated phenomena | 21/30 = 0,70 per report-scoring (1 + 9 + 11) — print only if needed | `POOLED.md` §2 |
| Bootstrap 95 % CI for D (percentile, resampling unit = report, n = 10, B = 10 000, `numpy.random.default_rng(20261012)`) | pass 1 [0,80; 0,98], pass 2 [0,65; 0,85], pass 3 [0,55; 0,77], **pooled [0,67; 0,87]** → «95 % ДІ 0,67–0,87»; mean errors pooled [0,60; 1,20] | `POOLED.md` §1–§3 |
| Sign test statistic (pre-registered primary outcome of protocol 37) | 10 wins of 10, **p = 0,5^10 = 1/1024 → «p = 2^−10 ≈ 0,001»** (the same number as §10, now printed with its derivation) | `OUTCOMES.json` `primary.sign_test` |
| κ verbal labels (Landis & Koch 1977 bands, cited as [LandisKoch1977] from v7) | pass 1–2 **0,44 помірна**; pass 2–3 **0,65 суттєва**; pass 1–3 **0,24 слабка**; original–F2 0,30 слабка (bands: 0,21–0,40 fair → «слабка», 0,41–0,60 moderate → «помірна», 0,61–0,80 substantial → «суттєва») | `POOLED.md` §4; `harness/kappa.py` `interpret()`; register row LandisKoch1977 |
| Implementation facts for «практичне впровадження» (author decision 2026-10-12: implementation only, no approbation/users claim) | Level 3 is implemented in the IntelliSim prototype (Go server core, Vue web client); the `modules/simulations/analysis_*.go` files (23 files) in the public bundle `intellisim-experiment-artifacts` (CITATION.cff `version: 0.1.2`, `date-released: 2026-08-08`, repository-code https://github.com/The-Maxwell/intellisim-experiment-artifacts — private until the author flips it) are **byte-identical** to core `5072b5f` (EXP-H2 engine) and, per §10 «Level-3 code identity», to core `6bb5bd9` (EXP-H engine) → print «версія 0.1.2», «між серіями липня та вересня 2026 р. код рівня 3 не змінювався», «22 звіти» generated through the engine (12 EXP-H + 10 F2; the 10 no-data A reports were produced by the standalone replica outside the metasystem, D24 — never say «всі 32» for the engine) | `cmp` of the 23 files, 2026-10-12; `intellisim.core` tag `0.1.2` = `55a7a6b`, HEAD `5072b5f` is one licence-only commit later |
| Never print | a pooled value without «апостеріорно»; a CI as if pre-registered; the pooled 77,8 % as the headline instead of the pre-registered 90 % + range; «впроваджено», «апробовано», user counts | author decisions 2026-10-12 |
