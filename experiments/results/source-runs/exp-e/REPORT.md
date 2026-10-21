# EXP-E — Level-2 strategy study & N×F / A×F cost-model validation: REPORT

**Date:** 2026-07-24 · **Protocol:** `experiments/30-evacuation/35-strategy-study-protocol.md` · **Verdict: cost model CONFIRMED (C1–C3 pass); behavioral hypothesis (C4, exploratory) supported for distribution-mode arms**

- Model: frozen V2 SFPE room (EXP-C reference, `6a5bb24f06d0910a77424b0c`) → 5 LLM-decision variants authored (D38): `exp-E-{decision, individual, A2, hybrid, archetype_full}` (`reference-model/model-ids.json` + snapshots). `none` reuses the frozen V2 (rules-only) directly.
- Engine: core `6bb5bd9`. LLM target **gpt-4.1-mini** (decision arms) / **gpt-4o** (archetype_full full-control, needs `simulation_execution`), set as the model-level execution target (D38 workaround). Pricing verified 2026-07-24: gpt-4.1-mini $0.40/$1.60, gpt-4o $2.50/$10.00 per 1M in/out.
- Reproducible: `python -m harness.expe_stats` + `python -m harness.expe_figures`. Raw: `results/exp-e/<condition>/run-<seed>/{run.json, export.csv.gz, metrics.json}`.
- **Determinism gate (none arm): PASS** (seed 301 twice → byte-identical; `determinism/result.json`). LLM arms are nondeterministic by construction (sampling).

## Conditions & reps

| Condition | Strategy (type × agent-type) | Reps | Seeds | Target |
|---|---|---|---|---|
| none | rules-only | 5 | 301–305 | — |
| archetype_decision | decision × archetype (A=1, 100 members) | 5 | 301–305 | gpt-4.1-mini |
| archetype_decision-A2 | decision × 2 archetypes (50/50) | 5 | 301–305 | gpt-4.1-mini |
| individual_decision | decision × agent (100 individuals) | 5 | 301–305 | gpt-4.1-mini |
| hybrid | 10 individual leaders + 90 archetype members | 5 | 301–305 | gpt-4.1-mini |
| archetype_full | full-control × archetype (A=1) | 2 | 301–302 | gpt-4o |

`individual_full` excluded (cost ≫ budget, per protocol). DecisionFrequency F = 5 (due steps D(T)=⌊T/5⌋); menu {proceed_to_exit, follow_crowd, wait_briefly, seek_alternate_route}; ArchetypeMode = distribution. All 27 runs finished `condition_met` (no censoring, no cap hits).

## Part 1 — Cost model (§4.1, §5.1–5.3)

### Per-run measurements (mean over reps)

| Condition | T | API calls | consult. (measured=calls) | pred. (attrition-corr.) | ratio m/pred | grouping (members/call) | tokens | $/run |
|---|---|---|---|---|---|---|---|---|
| none | 46 | 0 | 0 | 0 | — | — | 0 | $0 |
| archetype_decision | 92 | 18.8 | 18.8 | 19.0 | **0.99** | 40.4 | 138 149 | $0.057 |
| archetype_decision-A2 | 95 | 36.0 | 36.0 | 39.2 | **0.92** | 20.3 | 143 030 | $0.060 |
| individual_decision | 46 | 512.4 | 512.4 | 491.6 | **1.04** | 0.96 | 305 722 | $0.135 |
| hybrid | 90 | 66.8 | 66.8 | 64.0 | **1.04** | 10.4 | 147 286 | $0.062 |
| archetype_full | 46 | 14.5 | 14.5 | 10.0 | 1.45 | — (full-control) | 88 895 | $0.227 |

Machine verdicts: `stats.json`. Full per-run data: `<condition>/run-*/metrics.json`.

> **Correction, 2026-08-04.** The two token figures above were wrong in the original report
> and are now re-derived from the raw per-run files. `hybrid` read **141 249**; the mean of
> `hybrid/run-30{1..5}/run.json → token_usage.total.total_tokens`
> (172 193 / 144 396 / 128 355 / 137 473 / 154 011) is **147 286**. `archetype_full` read
> **71 565**, which is verbatim the total of `run-302` alone; the two-run mean of
> 106 224 and 71 565 is **88 895** — a single run had been copied in place of the average.
> `stats.json` (`total_tokens_mean`) was correct throughout, as was the article's Table 3,
> so only this report changed. No measurement was re-run and no raw file was touched. This
> closes the discrepancy recorded as unreconciled in `article/draft/COMPLIANCE.md`.
>
> **Correction, 2026-08-16.** The `archetype_full` $/run figure above read **$0.116**, which
> priced only the `simulation_decision` purpose: `metrics.json`'s `prompt_tokens` /
> `completion_tokens` omit full control's movement and property purposes (51 980 / 335 for
> run-301 versus the true `run.json → token_usage.total` split of 105 634 / 590), even though
> its `total_tokens` was complete. Re-priced from `run.json` totals at gpt-4o list rates:
> run-301 $0.270, run-302 $0.184, mean **$0.227**. `harness/expe_stats.py` now prices from
> `run.json` (single-purpose decision arms are unchanged by the fix), `stats.json` was
> regenerated (`cost_mean_usd` 0.116 → 0.227, `total_study_cost_usd` 1.803 → **2.025**), and
> the study total now matches the OpenAI billing cross-check below (≈$2.02 at list) exactly.
> No measurement was re-run and no raw file was touched.
>
> Wall-clock per run, added to the article's Table 3 on the same date, is not tabulated here;
> it is `metrics.json → active_duration_ms` (engine active run time, pauses excluded), whose
> per-condition means are 2.2 / 47.4 / 48.7 / 579.2 / 73.7 / 150.0 s in the row order above
> and are already carried in `stats.json` as `active_s_mean`.

### Verdicts vs pre-registered criteria (§0)

- **C1 — measured logical consultations within ±10% of prediction — PASS (4/4 core arms).** archetype_decision 0.99, A2 0.92, individual 1.04, hybrid 1.04 — all within ±10% of the **attrition-corrected** prediction. **Measurement nuance (pre-declared, D38):** the engine exposes API `calls`, not a separate logical-consultation counter, and agents *exit* mid-run, so the constant-N product N×D(T) is an upper bound. We therefore validate the N×F / A×F structure PER DUE STEP against the *live* population: individual issues one call per alive agent per due step, archetype issues A calls per due step. The attrition-corrected prediction (Σ over due steps of live population, resp. A) matches measurement to within ±10%. archetype_full (exploratory, n=2) measured 1.45× the pure decision count — full-control issues additional per-purpose calls (movement + properties) beyond the single decision consultation; reported as an exploratory observation, not a core-criterion result.
- **C2 — archetype ≈ 1/N of individual — CONFIRMED.** Per due step, archetype issues ~1 call while individual issues one per live agent (mean ≈ 55 during the run, 100 at the first due step). Ratio (per-due) = 0.018 = 1/54.5; at full population (first due step) the ratio is exactly 1/N = 0.01. The **grouping factor** makes the amortization explicit: archetype_decision resolves **40.4 member decisions per LLM call** (A2: 20.3; hybrid: 10.4) versus individual's **0.96 ≈ 1** (no amortization). This is the headline efficiency result — archetype scope amortizes one consultation across the whole population.
- **C3 — A=2 ≈ 2× A=1 — PASS.** Per-due-step calls: A1 = 1.04, A2 = 1.89 → ratio **1.81** (target 2.0, ±20% band). A×F linearity confirmed.

## Part 2 — Behavior (§4.2, §5.4) & robustness (§4.3, §5.5)

### C4 — behavioral hypothesis (EXPLORATORY, n=5, honestly labeled)

**Distribution-mode LLM decision strategies increase egress-time variance vs `none`; individual and full-control do not.**

| Condition | per-agent egress mean±SD (s) | across-run T_total spread (SD of T, steps) | dominant behavior |
|---|---|---|---|
| none | 6.7 ± **3.3** | **0.0** | rules: all move to exit |
| individual_decision | 6.8 ± **3.3** | **0.0** | LLM: 100% `proceed_to_exit` |
| archetype_full | 6.8 ± **3.2** | **0.0** | full-control: all move toward exit |
| archetype_decision | 10.8 ± **6.4** | 4.1 | diverse: ~59% proceed / 29% follow / 8% wait / 5% alt |
| archetype_decision-A2 | 10.4 ± **6.2** | 16.0 | diverse (same menu, 2 archetypes) |
| hybrid | 9.9 ± **5.8** | 19.1 | diverse (members) + proceed (leaders) |

Both spread measures (within-run per-agent SD ≈ **2×**, across-run T_total SD from 0 → 4–19) are higher for the three distribution-mode arms and unchanged for the homogeneous arms. **Interpretation:** when the optimal choice is unambiguous ("evacuate"), *individual* per-agent consultation and *full-control* both converge on the rules-only optimum (egress ≈ `none`, T=46) — at 27× and 1× the call cost respectively; behavioral richness (hesitation, crowd-following, waves) arises specifically from the **archetype distribution mode**, which samples a weighted label mix across members. The reusable behavioral baseline is EXP-C's 30-rep ensemble (T_total 13.2–13.5 s ≈ 44–45 steps), consistent with the EXP-E `none` arm (T=46).

### C5 — fallback rate <5% — NOT DIRECTLY MEASURABLE (D38).

The engine does not expose a fallback counter (verified in code). Proxy evidence of run health: all 27 runs finished `condition_met`; every decision step produced recorded labels for all live agents (no empty-decision steps); no run raised an execution error or invalid-target rejection. No degraded reps.

## Cost/behavior trade-off (headline)

`individual_decision` costs **27× the API calls** (512 vs 19) and **2.2× the tokens** of `archetype_decision`, yet — for this unambiguous-objective scenario — produces behavior that *collapses onto the rules-only baseline* (all agents proceed, T=46). `archetype_decision` delivers the behavioral diversity (the interesting, higher-variance egress) at **1/40 the per-call cost** via population amortization. This is the practical statement of the article's cost model: **archetype-scope decision strategies dominate the cost/behavior frontier for population-homogeneous scenarios.** Whole-study spend: **$2.02** at OpenAI list price (the pre-correction engine estimate read $1.80 — see the 2026-08-16 correction above) — « $15 cap (D32).

**Independent billing cross-check (`results/_billing/`, refreshed 2026-07-24 after EXP-D E2 + EXP-H):** OpenAI's own usage export confirms the engine accounting **exactly** for gpt-4.1-mini. EXP-E's **3 168** decision-arm requests are 3 168 of the now-**3 427** total gpt-4.1-mini requests (the other 259 = EXP-D E2), and the EXP-E gpt-4.1-mini list-price cost ($1.570) matches the per-condition engine sum. The engine under-attributes the gpt-4o full-control arm (~$0.23 vs ~$0.45 list), giving the $1.80 vs $2.02 gap. There was **zero cached input** on gpt-4.1-mini, so the hypothesized individual_decision prompt-caching benefit did not materialize. The whole-window OpenAI-billed amount is now **$5.8177** (posted 2026-07-24, `incentivized-tier` discount ≈ 60% of the ~$9.73 list; supersedes the earlier pre-E2/H $0.4999 partial) — driven mostly by EXP-D E2's gpt-4.1-mini input. Full reconciliation: `results/_billing/reconciliation.md`.

## Figures

- `figures/exp-e-cost-vs-strategy.{pdf,png}` — mean API calls (log scale) + tokens per condition; the N×F explosion of individual (512) vs archetype (19) / A2 (36) / hybrid (67).
- `figures/exp-e-label-share.{pdf,png}` — decision-label share per decision arm; individual = 100% proceed, archetype/hybrid = diverse.
- `figures/exp-e-egress.{pdf,png}` — per-agent egress-time distribution per condition; distribution-mode arms wider/higher.

## Deviations & findings

- **D38 (metasystem finding, worked around):** `POST /run`'s exec-target override is validated against `simulation_execution` unconditionally, rejecting decision-capable mini targets; used the model-level execution target instead (physics-identical). Candidate enabler U10; not fixed (no physics impact, clean workaround).
- **Cost-model nuance (D38):** consultations validated per-due-step against live population (agents exit); the constant-N product is an upper bound. No goalpost move — the N×F / A×F *structure* is confirmed.
- **C5 unmeasurable:** no engine fallback counter; reported via completeness proxy.
- **archetype_full (exploratory):** full-control on gpt-4o issues ~1.45× the pure decision-consultation count (extra movement/property purposes); egress collapses to the rules optimum (T=46) — the full-control column of the 2×2 continuum, honestly exploratory (n=2). `individual_full` excluded by cost per protocol.
- **Strategy authoring (D38):** engine has no archetype_/individual_ enum; scope derives from agent `type`; the decision menu is derived from `decision`-condition values. Two runs (`archetype_decision`, `individual_decision`) are retained for EXP-H reuse (§7.6).

## Artifact accounting (D20)

EXP-E entities registered in `results/ARTIFACTS.md`: 6 models (decision/individual/A2/hybrid/archetype_full + 1 superseded A2 orphan from a failed PUT, kept), per-condition base sims + 27 rep sims (4 pilot @301 + 5×4 core + 5 hybrid + 2 archetype_full) + 2 determinism sims. Nothing deleted.
