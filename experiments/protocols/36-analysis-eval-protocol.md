> **Provenance:** verbatim copy of `experiments/30-evacuation/36-analysis-eval-protocol.md` from `../intellisim.experiments` @ `a9694df`, copied 2026-09-08 for the Level-3 article campaign (`intellisim.experiments.l3`). Paths inside refer to the sibling repo; here the same material lives under `results/exp-h/`, `results/_billing/`, `experiments/`. Do not edit; corrections are recorded in `results/_article/KEY-NUMBERS.md`.

# EXP-H — Level-3 analysis validity against known ground truth (protocol)

> **Purpose.** Evaluate whether Level-3 reasoning analysis detects phenomena whose ground truth is *already established by the benchmark experiments* — answering the LLM-judge-circularity critique [Larooij2025] by scoring reports against measured/benchmark truth, not another LLM's opinion.

**Requires:** completed source runs from EXP-B/C/E/F/G (Phase 4 timing); Level-3 pipeline live (`POST /simulations/:id/analyses`, async 202 → SSE `analysis.completed` / poll; PDF via GridFS); o4-mini (default) and o3 (stretch) reachable via the provider. No engine upgrades beyond those the source experiments needed.

**Related docs:** source protocols [`33-sfpe-protocol.md`](33-sfpe-protocol.md), [`32-rimea-protocol.md`](32-rimea-protocol.md), [`35-strategy-study-protocol.md`](35-strategy-study-protocol.md), [`../40-wildfire/41-protocol.md`](../40-wildfire/41-protocol.md), [`../50-epidemic/51-protocol.md`](../50-epidemic/51-protocol.md); ground-truth numbers [`../../article/03-baselines-and-benchmarks.md`](../../article/03-baselines-and-benchmarks.md) §3/§6/§7; critique positioning [`../../article/01-related-work.md`](../../article/01-related-work.md) §5.

**Deviations must be logged in `DECISIONS.md`** ([`../../DECISIONS.md`](../../DECISIONS.md)).

---

## 1. Design

~10 analysis reports are generated over completed campaign runs whose key phenomena are *known* (measured in their own experiments and anchored to benchmarks). Each report is scored on a pre-filled ground-truth sheet: did the analysis find what is provably there, did it state anything the data contradicts, did it invent phenomena. Level-3 internals (deterministic evidence assembly → sectioned reasoning calls → synthesis; report = sections/warnings/conclusions/meta + PDF) are treated as a black box.

### 1.1 Inputs — the report set (10 primary + 2 stretch)

Five scenario pairs; **within each pair one report in `en`, one in `uk`** (balances language across scenario types and tests the Cyrillic pipeline end-to-end). `llm_target = o4-mini` for all primary reports.

| # | Source run (sim to analyze) | Ground-truth phenomena (pre-filled from) | Lang |
|---|---|---|---|
| 1 | EXP-C room evacuation — the median-T_drain rep | Door congestion/queue; flow-limited (≈constant-rate) egress; total egress time on the pre-registered SFPE band (order 10² s — exact band per the door choice in `33-sfpe-protocol.md` §3; the baselines §3 1.0 m worked example is the ≈110 s reference scale) | en |
| 2 | EXP-C — a second rep (fastest T_drain) | Same phenomenon list, that rep's measured values | uk |
| 3 | EXP-B Test 12 (bottleneck, 150 agents) | Congestion localized in Room 1 (not Room 2); corridor as limiting element; clearance time (measured) | en |
| 4 | EXP-B Test 4 run (congestion at opening) — if Test 4 was not executed (stretch battery), substitute a second Test-12 rep and log the substitution | Density-dependent slowdown / sustained flow at the opening (measured values) | uk |
| 5 | EXP-E `archetype_decision` run | Egress slower / more spread than the rules-only reference (EXP-E + EXP-C measured values); non-degenerate decision-label mix incl. hesitation (`wait_briefly`) | en |
| 6 | EXP-E `individual_decision` run | Same list, that run's measured values | uk |
| 7 | EXP-F wildfire, sub-critical density **p = 0.50** | Fire dies out; small final burned fraction (measured); no burn-through to the far edge — sub-critical regime relative to p_c ≈ 0.59 [Percolation][NL-Fire] | uk |
| 8 | EXP-F wildfire, super-critical density **p = 0.70** | Near-complete burn (measured fraction); burn-through across the grid | en |
| 9 | EXP-G epidemic, medium-transmissibility run | Single epidemic peak (measured height + timing); final size (measured, vs analytic anchor [SIR-finalsize]); S non-increasing / R non-decreasing | en |
| 10 | EXP-G — a second rep of the same point | Same list, that rep's values | uk |
| 11–12 (stretch) | Re-run pair #1–2 (same runs, same context, same languages) with `llm_target = o3` | Same sheets — o3 vs o4-mini comparison | en+uk |

If EXP-F/EXP-G sweeps used different grid points, pick the executed points closest to 0.50/0.70 (log the substitution). Runs are referenced by sim ID + run ID in the manifest; **source sims must not be deleted** (RUNBOOK §3).

### 1.2 Context messages (verbatim templates — do not embellish)

The context must be **neutral and non-leading**: a domain hint only, **never** naming the expected phenomena. Banned words in any hint (en/uk): congestion, bottleneck, queue, arching, threshold, critical, percolation, peak, final size, faster/slower, затор, черга, вузьке місце, поріг, критичн-, пік, фінальний розмір.

- **en template:** `Analyze this simulation's results. Context: {hint} Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.`
- **uk template:** `Проаналізуйте результати цієї симуляції. Контекст: {hint} Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.`

| Scenario | {hint} en | {hint} uk |
|---|---|---|
| EXP-C (#1–2) | `People are evacuating a single room through one door.` | `Люди евакуюються з одного приміщення через одні двері.` |
| EXP-B (#3–4) | `150 people move out of a building section consisting of two rooms connected by a corridor.` (Test 4 variant: `People pass through a wide opening at varying crowd densities.`) | `150 людей виходять із секції будівлі з двох кімнат, з'єднаних коридором.` (Test 4: `Люди проходять крізь широкий отвір за різної щільності натовпу.`) |
| EXP-E (#5–6) | `People are evacuating a room through one door; some agent decisions during the run are made by a language model.` | `Люди евакуюються з приміщення через одні двері; частину рішень агентів під час виконання ухвалює мовна модель.` |
| EXP-F (#7–8) | `Fire spreads through a forest of trees.` | `Вогонь поширюється лісом.` |
| EXP-G (#9–10) | `An infectious disease spreads in a population of moving agents.` | `Інфекційне захворювання поширюється в популяції рухомих агентів.` |

API call per report: `POST /simulations/:id/analyses` with `{context_message, language: en|uk, llm_target, selection}` — `selection` left at its default (the run listed in §1.1) unless the engine requires an explicit run selection; record the exact request body per report.

## 2. Rubric and scoring sheet

Each report has a ground-truth sheet with **exactly 3 core phenomena** (the bold-faced lists in §1.1 are the sheet seeds; each sheet states the phenomenon, the measured value(s) with source file paths, and the acceptable range = the value's data-supported range, not the SFPE/analytic band).

**Per phenomenon:** `2` = explicitly identified **and** quantitatively characterized correctly (numbers within the data-supported range) · `1` = partially/qualitatively identified (or quantitative but imprecise) · `0` = missed or wrongly characterized.

**Per report, additionally counted and each instance quoted verbatim in the sheet:**
- **Factual errors** — statements contradicting the exported data/statistics (checkable against `results/**`).
- **Hallucinated phenomena** — asserted patterns/mechanisms with no support in the data. Hedged speculation explicitly marked as such by the report is neither an error nor a hallucination.
- **Usefulness:** actionable recommendations present (y/n, quote one).

**Scorer:** the author, against the pre-filled sheet. Optional inter-rater check: a second scorer scores independently; report Cohen's κ on the 0/1/2 items (note as optional — absence is a stated limitation).

**Blinding mechanism:** all 10 ground-truth sheets are fully pre-filled and **committed to git before the first analysis is generated** (commit timestamp is the pre-registration evidence). Scores must cite verbatim report quotes as evidence.

### 2.1 Scoring-sheet template (one per report → `results/exp-h/report-N/scoring.md`)

```markdown
# EXP-H scoring — report N
Meta: sim_id=… run_id=… analysis_id=… lang=… llm_target=… generated=… scorer=… scored=…
Request body: {…verbatim…}

| Phenomenon (pre-filled) | Ground truth (pre-filled: value ± range, source path) | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|---|
| 1. … | … | | |
| 2. … | … | | |
| 3. … | … | | |

Factual errors (verbatim, one per line + what the data says): …count = X
Hallucinated phenomena (verbatim + why unsupported): …count = Y
Actionable recommendations: y/n — quote: "…"
Language/format notes (uk rendering, PDF/Cyrillic issues, section completeness, warnings shown): …
```

## 3. Pre-registered success criteria

| # | Criterion | Threshold |
|---|---|---|
| 1 | Phenomenon points achieved across the 10 primary reports (max = 10 × 3 × 2 = **60**) | **≥ 70%** (≥ 42 points) |
| 2 | Factual errors | **mean ≤ 1 per report** (≤ 10 total across primary reports) |
| 3 (reported, no threshold) | Hallucinated-phenomenon count; usefulness rate; en-vs-uk and o4-mini-vs-o3 score comparisons | Descriptive |

No per-report minimum is pre-registered; per-report scores are all published (a weak report is data, not a protocol failure).

## 4. Procedure

0. **Preconditions.** Source experiments complete with committed raw results; the §1.1 run list resolved to concrete sim/run IDs in `results/exp-h/manifest.json`; provider up with o4-mini visible.
1. **Pre-fill all 10 ground-truth sheets** from measured results (+ benchmark anchors) and **commit** (blinding gate, §2).
2. **Pilot — report #1 only.** Generate → wait (SSE `analysis.completed` or poll `GET /simulations/:id/analyses/:aid`) → fetch JSON detail + PDF (`GET …/analyses/:aid/file`) → verify pipeline health: all sections present, PDF renders, warnings/conclusions/meta populated; record token usage + $; project cost × 12; if the projection is unacceptable (o4-mini reasoning tokens dominate; consult the user if projected total exceeds the remaining campaign LLM budget), stop and log before continuing.
3. **Generate reports #2–10** (sequential; #2 first to confirm the uk/Cyrillic path early). Save per report: `report.json` (full detail incl. sections/warnings/conclusions/meta + token usage), `analysis.pdf`, request body.
4. **Stretch:** reports #11–12 on o3 (budget permitting; log the decision).
5. **Score** every report per §2 (author; optional second scorer independently).
6. **Aggregate**: points table, error/hallucination tallies, criteria verdicts, en/uk and o3 comparisons, cost table (tokens incl. reasoning + $ per report) → `results/exp-h/REPORT.md`. Update `STATUS.md`; commit.

## 5. Outputs

```
results/exp-h/manifest.json                      (run list, request bodies, model targets, engine/harness commits, dates)
results/exp-h/ground-truth/sheet-N.md            (pre-filled, committed BEFORE generation — blinding evidence)
results/exp-h/report-N/{analysis.pdf, report.json, scoring.md}    N = 1…10 (+11–12 stretch)
results/exp-h/REPORT.md                          (aggregate scores, verdicts vs §3, deviations, cost)
```

## 6. Cost note

~10 primary analyses on **o4-mini** (reasoning model; per-analysis cost driven by sectioned reasoning calls + synthesis over the deterministic evidence bundle) + up to 2 stretch on **o3** — strong models are justified here per D3 (Level-3 is one of the two "scientifically necessary" uses). Exact per-analysis cost is measured at the pilot (step 4.2) from the engine's token accounting; no battery proceeds without that number. `TODO-VERIFY: current OpenAI pricing for o4-mini / o3 at execution date (record rates used in the manifest).`

## 7. Deviations

Any change to the run list, hints, rubric, thresholds, language split, or model targets after the sheets are committed → dated row in [`../../DECISIONS.md`](../../DECISIONS.md) + 🔶 in `STATUS.md`. Editing a ground-truth sheet after its report exists is prohibited; corrections require a new dated sheet version with the original preserved.
