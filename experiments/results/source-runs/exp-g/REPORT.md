# EXP-G — Epidemic (spatial SIR) generality study: REPORT

**Date:** 2026-07-24 · **Protocol:** `experiments/50-epidemic/51-protocol.md` · **Verdict: qualitative generality demonstrated; quantitative docking NOT achieved (documented divergence)**

- Baseline: NetLogo **epiDEM Basic** (Yang & Wilensky 2011) [NL-epiDEM], 51×51 torus, headless BehaviorSpace (`results/exp-g/netlogo/sweep/`, `epiDEM-50.nlogox`, BOM-less).
- IntelliSim: frozen reference `exp-g` spatial-SIR model (`6a610f08ff82d466bf2307b7`); as-built + freeze edits in `reference-model/model-frozen.json` + `edit-log.json`.
- Engine: core `6bb5bd9` (= `525dca0` + U9, D36 — zero engine-physics change). Determinism gate **PASS** (`intellisim/determinism/result.json`: same U1 seed twice → byte-identical I(t), SHA-256 `af764008…`).
- Raw data: `intellisim/point-{lo,med,hi}/raw/` (manifest + 90 gzipped step exports) + `curves.jsonl`; `netlogo/sweep/raw/behaviorspace.csv`. Stats/figures reproducible via `python -m harness.expg_stats` and `python -m harness.expg_figures`.

## Part 1 — Level-1 generation (EXP-A instance)

**5/5 attempts produced a valid, simulation-ready model in a single turn — zero structural-fix iterations.** Per **D33** (API-driven, server-side timestamps; not UI-timed, not pooled with EXP-A-evac). Turn-1 generation latency **7.2–11.4 s** (gpt-4.1); total tokens per attempt **3 585–3 984** (all reasoning_tokens 0). Every attempt's created simulation reached `ready`/`ok` with no repair turn. Evidence: `generation/attempt-{1..5}/summary.json` + transcripts; prompt in `51-protocol.md` §1.1.

**This is the reportable Level-1 contrast to EXP-F:** the SIR domain maps cleanly to the engine's rule/archetype vocabulary (susceptible/infected/recovered archetypes + proximity-gated `transform` + countdown recovery), so the LLM authored an executable model on the first try every time. EXP-F's fire-spread interaction, by contrast, was repeatedly authored as a free-text `custom` action and needed 1/1/1/2/1 repair turns. The two scenarios bracket the method's Level-1 behaviour: faithful when the target mechanic has a first-class engine primitive, one-repair-away when it does not.

Reference freeze: best attempt + minimal logged edits → frozen 3-archetype SIR (`reference-model/`). R₀ calibrated by index-tracing (Method 1, primary; **D35**): β 0.145/0.18/0.35 → R₀ ≈ **1.5 / 2.1 / 2.7** (`calibration/r0-scan.json`: β0.18→2.14±0.31, β0.35→2.71±0.34; R₀=3 unreachable at r=1.5 → high point capped at 2.7, D35).

## Part 2 — Transmissibility sweep + docking

**Design:** 3 transmissibility points (lo/med/hi) × 30 U1-seed reps/side; N=200, ~50×50 torus, `none` strategy, 2000-step cap. IntelliSim per-step hazard β with contact radius r=1.5; epiDEM per-encounter infection-chance 10/20/40 %. Docking pairs the **ordered** transmissibility levels (lo↔10, med↔20, hi↔40) since R₀-matching saturates (**D35**) — the pairing and its caveat are reported, not hidden.

**Execution:** IntelliSim 90/90 runs finished (30 per point), all burned out well under the cap (peak steps 19–38, extinction by ~120). epiDEM 90/90 via BehaviorSpace headless. Determinism gate IDENTICAL (above).

### Per-point results (final size = R(∞)/N)

| point | R₀ (index) | epiDEM chance | IS final mean ±SD [95% CI] | epiDEM final mean ±SD | analytic z | KS D | KS p (BH) | TOST ±0.05 |
|---|---|---|---|---|---|---|---|---|
| lo | 1.5 | 10 % | 0.123 ±0.090 [0.089, 0.157] | 0.057 ±0.017 | 0.583 | 0.60 | **0.000 (reject)** | not eq |
| med | 2.1 | 20 % | 0.115 ±0.100 [0.078, 0.153] | 0.070 ±0.031 | 0.822 | 0.33 | 0.071 (no reject) | not eq |
| hi | 2.7 | 40 % | 0.429 ±0.170 [0.366, 0.493] | 0.129 ±0.046 | 0.916 | 0.90 | **0.000 (reject)** | not eq |

Machine verdicts: `stats.json`. Peak infected fraction: epiDEM peaks at **t = 0** (subcritical — the ~10-agent seed recovers with almost no secondary spread), IntelliSim peaks at **t ≈ 19/28/38** (genuine self-sustaining outbreaks) → peak-height and peak-time CIs do not overlap at any point.

### Verdicts vs pre-registered criteria (§0)

**A. Analytic final-size check (SOFT — reported with discussion, not a hard gate).**
- **(a) Direction — PASS (3/3).** Analytic well-mixed z (0.583/0.822/0.916) lies **above** every observed 95% CI, for both tools. Spatial structure + finite N depress final size below the mean-field idealisation exactly as anticipated [SIR-finalsize].
- **(b) Magnitude — BREACH (0/3).** |z − mean(IS)| = 0.46 / 0.71 / 0.49, all ≫ 0.15. The depression is far larger than the soft ±0.15 tolerance because both models are strongly spatially constrained (r=1.5 contact neighbourhood on a torus with local random walk). This is the discussed breach the protocol anticipates ("well-mixed theory is an idealisation; discuss any breach rather than silently failing") — not a metasystem defect.
- **(c) Monotonicity (P2).** epiDEM strictly monotone (0.057 < 0.070 < 0.129). IntelliSim **not strictly** monotone: lo 0.123 vs med 0.115 (Δ = 0.008) — a statistical tie (fully overlapping CIs, SD ≈ 0.09–0.10) in the near-critical low-R₀ regime where most outbreaks fizzle; the hi point rises sharply (0.429) as expected. Reported as a tie, not a real inversion.

**B. epiDEM docking (distributional) — NOT achieved.** On the 30-vs-30 final-size distributions, Benjamini–Hochberg-corrected across the family: **KS non-rejection 1/3** (med only); reject at lo and hi. **TOST ±0.05 equivalence 0/3.** Peak-height and peak-time CIs do not overlap at any point. **Parity points (KS non-reject ∧ TOST equivalent): 0/3.**

**Overall:** IntelliSim reproduces the *structure and qualitative behaviour* of a spatial SIR epidemic — single-peaked I(t), burnout to extinction, monotone-ish rise with transmissibility, and the same strong sub-mean-field spatial suppression — but does **not** achieve quantitative distributional parity with epiDEM Basic at the paired transmissibility levels. The divergence is fully traced to two **pre-declared** modelling-semantics differences (below), not to a metasystem defect; the frozen IntelliSim model faithfully implements the natural-language specification. Per **D24**, no fix-and-rerun is triggered (no defect); the calibration is frozen (D35) and criteria are pre-registered, so no re-tuning is performed (no goalpost move).

### Why the two diverge (pre-declared differences, §6)

1. **Transmission construction (primary).** epiDEM applies a per-*encounter* infection-chance that scales with the number of infected contacts; IntelliSim applies a frequency-independent per-*step* hazard β that fires given ≥1 infected within r, regardless of contact count. Calibrated to equal *index-tracing* R₀ (a first-generation, fully-susceptible measurement), the two constructions produce different fully-developed epidemics: at chance 10–40 % epiDEM stays effectively subcritical in the spatial setting (peak at t=0, final ≈ seed fraction), while IntelliSim's hazard sustains modest outbreaks. Index-R₀ matching does **not** guarantee matched dynamics across different transmission micro-mechanisms — the central quantitative finding of EXP-G, and clearly visible in `figures/exp-g-It-mean-curves`.
2. **Initial-seed mismatch (secondary, newly quantified — D37).** IntelliSim seeds ~2 initial infected (matching the NL prompt; measured I(0) 2.2–2.7); epiDEM Basic's default outbreak seeds ~10 (measured I(0) 9.4–10.2, ~5 %). The §4 mapping pre-declared "2 infected (or matched %) — equal after Phase-1 match"; the match was not enforced. Direction of the effect is counter-intuitive (epiDEM has the larger seed yet the smaller final size), confirming the transmission-construction difference dominates; the seed gap is a documented secondary confound. Logged as **D37**; a future extended-version re-match (Safety Science/SMPT paper) can isolate the transmission effect.

### Figures

- `figures/exp-g-finalsize-vs-R0.{pdf,png}` — observed final size (both tools, mean ±95% CI) vs index-tracing R₀, with the analytic well-mixed z(R₀) curve. Shows both tools far below mean-field (spatial suppression) and IntelliSim above epiDEM, diverging at the high point.
- `figures/exp-g-It-mean-curves.{pdf,png}` — mean I(t)/N per point, both tools, with per-rep spaghetti. Visually contrasts epiDEM's subcritical decay-from-seed against IntelliSim's growing-then-burning-out outbreaks.

## Deviations & engineering findings

- **D33:** Part 1 API-driven (server-side timestamps), not Playwright-UI-timed.
- **D35:** ordered-level docking pairing (R₀-matching saturates; R₀=3 unreachable at r=1.5); index-tracing R₀ registered as primary.
- **D36 (U9):** engine `6bb5bd9` (startup-reconcile fix); non-physics code path; determinism unchanged.
- **D37 (new):** epiDEM ↔ IntelliSim initial-infected mismatch (~10 vs ~2) documented; §0.B docking recorded as **not achieved** with divergence attributed to pre-declared transmission-construction difference (primary) + seed mismatch (secondary). No re-tune (pre-registered criteria, frozen calibration).
- Part 1's zero-fix-iteration result live-validates that SIR maps cleanly onto engine primitives — the reportable contrast to EXP-F's `custom`-action gap.

## Artifact accounting (D20)

EXP-G metasystem entities registered in `results/ARTIFACTS.md`: 5 generation chats+models+sims (Part 1), reference model + calibration/R₀-scan sims, 3 per-point base sims + 90 sweep-rep sims, 1 determinism base + 2 determinism runs. Nothing deleted.
