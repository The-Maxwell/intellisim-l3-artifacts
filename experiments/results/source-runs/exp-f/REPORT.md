# EXP-F — Wildfire percolation generality study: REPORT

**Date:** 2026-07-24 · **Protocol:** `experiments/40-wildfire/41-protocol.md` · **Verdict: docking PASS (both pre-registered criteria)**

- Baseline: NetLogo **Fire** (Models Library), re-baselined at 61×61 per **D34** (`results/exp-f/netlogo/sweep-61/`, `Fire-61.nlogox`, BOM-less).
- IntelliSim: frozen reference `exp-F-fire-reference-61` (model `6a61096cff82d466bffc5105`, base sim `6a61096cff82d466bffc510b`); as-built + freeze edits: `reference-model/model-frozen-61.json` + `edit-log.json`.
- Engine: 157 sweep runs on core `525dca0`, 353 on `6bb5bd9` (= `525dca0` + **U9** startup-reconcile fix, D36 — zero engine-physics change; batteries deliberately NOT restarted).
- Raw data: `intellisim/sweep-61/raw/` (manifest + 510 gzipped step exports), `netlogo/sweep-61/raw/behaviorspace.csv`. Stats/figures reproducible via `python -m harness.expf_stats`.

## Part 1 — Level-1 generation (EXP-A instance)

5/5 attempts produced a valid, simulation-ready model (A1-style validity 5/5). Mechanics per **D33** (API-driven, server-side timestamps — not UI-timed; not pooled with EXP-A-evac). Turn-1 model-generation latency 11.7–14.9 s (gpt-4.1); total tokens per attempt 7 496–12 890. Structural-fix iterations after turn 1: **1/1/1/2/1** — the recurring turn-1 gap (all 5 attempts): the LLM authored the fire-spread interaction as a free-text `custom` action instead of an engine-executable `transform`; one repair turn fixed it every time (the reportable Level-1 finding for this scenario — contrast: EXP-G needed **zero** such iterations). Evidence: `generation/attempt-{1..5}/`, prompt in `generation/prompt.txt`.

Reference freeze: attempt-based 3-archetype transform chain (tree → burning → ash), completeness gate **PASS** at 101×101 (diamond wavefront, 4k rings, one ring/step, all-ash, `condition_met`; `completeness-gate/`), then **D34** re-baseline to 61×61 for the sweep (101² infeasible: ~14.7 s/step at 10 201 agents).

## Part 2 — Density sweep + docking

**Design (D34):** 61×61 world (3 721 cells), 17 densities {0.40, 0.45, 0.50, 0.55–0.65 step 0.01, 0.70, 0.75, 0.80} × 30 reps/side. Placement: harness-sampled **distinct** cells per (density, rep), seed = 40000 + 100·(100p) + rep (engine `random` placement samples with replacement — pre-approved fallback). Metric (both tools, exact): burned fraction = burned **non-left-column** trees ÷ non-left-column trees; percolation = burned presence in the rightmost column. Ignition: IntelliSim left-edge marker ignites col 0+1 at step 1 ≙ NetLogo t=0/t=1 effective set (≤1-step offset, duration-only).

**Execution:** IntelliSim 510/510 finished across 4 session stints (skip-logic resume; interrupted in-flight runs re-ran automatically; ~25 spent duplicates retained per D20). **All 510 runs ended `condition_met`** (fire extinguished; never hit the 400-step cap) — steps 6–206, mean 80.2. Pilot 9/9 + determinism gate **IDENTICAL** (same placement seed twice → same canonical series SHA-256; `intellisim/determinism/result.json`). NetLogo: 510/510 via BehaviorSpace headless, seeds per D34 XML.

### Per-point results (mean burned fraction ± SD; percolation probability)

| p | IS burned | NL burned | IS perc | NL perc | KS D | KS p (BH) | mean diff | TOST ±0.05 |
|---|---|---|---|---|---|---|---|---|
| 0.40 | 0.046±0.015 | 0.046±0.014 | 0.00 | 0.00 | 0.17 | 0.92 | −0.001 | eq |
| 0.45 | 0.064±0.020 | 0.060±0.019 | 0.00 | 0.00 | 0.13 | 0.96 | +0.004 | eq |
| 0.50 | 0.098±0.044 | 0.094±0.057 | 0.00 | 0.00 | 0.20 | 0.78 | +0.004 | eq |
| 0.55 | 0.199±0.102 | 0.255±0.134 | 0.00 | 0.07 | 0.27 | 0.67 | −0.056 | — |
| 0.56 | 0.254±0.145 | 0.280±0.154 | 0.13 | 0.10 | 0.27 | 0.67 | −0.027 | — |
| 0.57 | 0.343±0.137 | 0.265±0.165 | 0.17 | 0.10 | **0.40** | 0.27 | +0.078 | — |
| 0.58 | 0.395±0.180 | 0.344±0.159 | 0.30 | 0.27 | 0.23 | 0.67 | +0.051 | — |
| 0.59 | 0.515±0.215 | 0.455±0.215 | 0.47 | 0.30 | 0.23 | 0.67 | +0.060 | — |
| 0.60 | 0.586±0.227 | 0.640±0.214 | 0.67 | 0.70 | 0.17 | 0.92 | −0.054 | — |
| 0.61 | 0.681±0.207 | 0.683±0.228 | 0.80 | 0.70 | 0.20 | 0.78 | −0.003 | — |
| 0.62 | 0.802±0.132 | 0.740±0.184 | 0.93 | 0.83 | 0.23 | 0.67 | +0.062 | — |
| 0.63 | 0.854±0.088 | 0.820±0.159 | 1.00 | 0.93 | 0.27 | 0.67 | +0.034 | — |
| 0.64 | 0.872±0.096 | 0.893±0.067 | 0.97 | 1.00 | 0.20 | 0.78 | −0.021 | — |
| 0.65 | 0.878±0.152 | 0.931±0.031 | 0.93 | 1.00 | 0.23 | 0.67 | −0.053 | — |
| 0.70 | 0.976±0.008 | 0.975±0.009 | 1.00 | 1.00 | 0.13 | 0.96 | +0.000 | eq |
| 0.75 | 0.991±0.003 | 0.992±0.003 | 1.00 | 1.00 | 0.27 | 0.67 | −0.001 | eq |
| 0.80 | 0.997±0.002 | 0.997±0.001 | 1.00 | 1.00 | 0.27 | 0.67 | −0.001 | eq |

Full tables: `analysis/curves.csv`, `analysis/ks-tost-table.csv`; machine verdicts: `stats.json`; IS point summary: `intellisim/sweep-61/metrics.json`.

### Verdicts vs pre-registered criteria (§0)

1. **Transition-midpoint agreement — PASS.** Logistic fits (primary): p_mid IntelliSim **0.5909** (k 77.6, boot CI95 [0.589, 0.592]) vs NetLogo **0.5956** (k 74.8, CI95 [0.5907, 0.5998]) → |Δ| = **0.0046 ≤ 0.02**. Both midpoints flank the theoretical p_c ≈ 0.5927 [Percolation] (secondary observation — finite-size 61×61 crossing; proximity not gated).
2. **Distributional docking of burned fraction — PASS.** Two-sample KS per point (30 vs 30), Benjamini–Hochberg across the 17-point family: **17/17 non-rejections** (≥14 required). Max divergence at p = 0.57 (D = 0.40, raw p = 0.016, BH-adjusted p = 0.27) — inside the transition window where run-to-run variance peaks.
3. **Secondary TOST (±0.05 absolute, non-gating):** 6/17 points formally equivalent — the 6 points outside the transition window (p ≤ 0.50, p ≥ 0.70) where SDs are small; within the window (0.55–0.65) per-point SDs of 0.09–0.23 make ±0.05 mean-equivalence unresolvable at n = 30 (CIs span the margin), while the KS battery (criterion 2) shows the distributions themselves are statistically indistinguishable. Reported as registered; no goalpost move.

### Figures

- `figures/exp-f-percolation-curves.{pdf,png}` — percolation probability vs density, both tools + logistic midpoints + p_c reference line.
- `figures/exp-f-burned-fraction.{pdf,png}` — mean burned fraction ±1 SD envelopes, both tools.

## Deviations & engineering findings

- **D33:** Part 1 API-driven (server-side timestamps), not Playwright-UI-timed.
- **D34:** 61×61 fallback both tools; harness-manual distinct-cell placement; exact burned-fraction/percolation metric definitions; ignition equivalence; BOM-less `.nlogox` requirement.
- **D36 (U9):** core could not start on the D20-grown database (startup reconcile > FX 15 s budget) — fixed via status-filtered listing on `feature/experiment-enablers` `6bb5bd9`; mid-sweep engine-commit change is a non-physics code path, determinism semantics unchanged (sweep NOT restarted).
- Session-interruption robustness: the sweep survived 3 kill/resume cycles (session reaping + user stops) purely on manifest skip-logic; interrupted in-flight sims were stopped via the control API and re-ran automatically — all spent entities retained + registered (D20).
- All-`condition_met` termination (510/510) live-validates U2 population-condition termination at scale.

## Artifact accounting (D20)

~535 EXP-F metasystem entities registered in `results/ARTIFACTS.md`: 5 generation chats+models (+2 fix-turn descendants each), reference model + base sim, completeness-gate entities, 9 pilot sims, 2 determinism sims, 510 sweep sims + ~25 interrupted/spent duplicates. Nothing deleted.
