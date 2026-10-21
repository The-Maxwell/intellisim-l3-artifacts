# EXP-C — SFPE hydraulic closed-form validation — REPORT

Protocol: `33-sfpe-protocol.md`. Date: 2026-07-18. Engine: core `525dca0` (clean, manifests).
Model: frozen `exp-evac-V2-reference` (`6a5bb24f06d0910a77424b0c`, as-built `30-model-spec.md`
§10, D27): 40×40 @ 0.4 m/cell, interior 38×38 = 15.2 m, **door 1.2 m (3 cells, pre-registered
primary)**, N=100 @ 1.0 cell/step, block collision, U2 termination `population==0`.
**U5 config deviation (D29):** engine defaults (revert, 4 iters) instead of the protocol's
"best_effort + tuned max_iterations" — calibration (D27) showed defaults drain fully with zero
stragglers post aim-point fix; no tuning was needed. Steps cap 900. Raw: `raw/` (manifest, 30
exports + snapshots), pilot in `pilot/raw/`.

## Pilot & pilot-gate deviation (D29)

Pilot (seeds 11–13): mechanically clean — `condition_met`, monotone drain, 0 penetrations —
but T_drain 12.0–13.5 s vs the §3 band 72.6–98.3 s: the §5.1 "within ~2× of band center" gate
FAILED for the root cause already quantified at calibration (D27): the lattice crowd has no
speed–density coupling and the doorway has zero dwell (removal triggers on body-overlap with
the door zone, reaching one cell past the plane). A fundamental-diagram/headway model is a
research-scale engine feature, out of campaign scope — per §10 the deviation was logged and the
main battery executed anyway to QUANTIFY the deviation; criteria are evaluated exactly as
pre-registered (no goalpost moves).

## Main ensemble (30 reps, gen seeds 101–130, duplicate+regenerate)

- 30/30 `condition_met`; **0 censored**; wall penetrations at final steps: **0**.
- Determinism evidence: seed 101 run twice (det-a/det-b) → **byte-identical exports**
  (SHA-256 `851d500eeba1a092…`). Spread across reps = initial-placement variation only.
- Per-rep metrics: `metrics.json` (T_first, T_total, T_drain, F̂c, F̂s, v̂_free, integrity).

| Metric | Mean | 95% CI (t, df=29) | min–max |
|---|---|---|---|
| T_first (s) | 0.67 | 0.55–0.79 | 0.3–1.5 |
| T_total (s) | 13.64 | 13.52–13.76 | 13.2–14.1 |
| **T_drain (s)** | **12.97** | 12.81–13.13 | 12.0–13.8 |
| **F̂s (P/(m·s))**, We = 0.9 m | **11.34** | 11.09–11.59 | 10.1–12.3 |
| **v̂_free (m/s)**, §4 path-sum metric | **1.398** | 1.380–1.416 | 1.37–1.43 |

## Verdicts (pre-registered §6, CI-within-band rule)

| # | Criterion | Band | Result | Verdict |
|---|---|---|---|---|
| 1 | mean T_drain | 72.6–98.3 s | 12.97 s (×6.6 fast) | **FAIL** |
| 2 | mean F̂s | 1.0–1.4 (target 1.2–1.3) | 11.34 | **FAIL** |
| 3 | mean v̂_free | 1.19–1.34 m/s | 1.398 (path-sum) | **FAIL** (see note) |
| 4 | mean T_total vs T_first + 100/Fc (86.2 s ± 15%) | 73.2–99.1 s | 13.64 s | **FAIL** |

**Criterion-3 note (measurement artifact, declared):** the §4 metric sums per-step Euclidean
displacements of integer positions — diagonal walks zigzag under rounding and inflate path
length ~5%. The endpoint-displacement measure over the same window gives **1.344 m/s**
(calibration: 1.008 cells/step), i.e. the commanded 1.333 m/s within 0.8% — brushing the
band's 1.34 upper edge. Speed realization is exact (EXP-B T1: 0.0% dev; T7: ≤1.36%); the
criterion fails on the metric definition, not on speed fidelity.

## Analysis (for the article's adequacy discussion)

- **Qualitative SFPE patterns hold:** egress is flow-limited (drain rate constant within ±5%
  across reps and within each run — F̂c CV ≈ 2.6%), a queue/arch forms at the door, and total
  time decomposes as T_first + drain exactly (criterion 4's failure is inherited entirely from
  the flow constant, not the accounting).
- **The quantitative flow constant is ~8.7× SFPE's Fs,max.** Mechanism: (i) the lattice crowd
  advances shoulder-to-shoulder at full speed — no speed–density coupling (fundamental
  diagram absent by construction; the model spec's declared "no social forces" limitation);
  (ii) doorway dwell is zero — removal fires when the body overlaps the exit zone (one cell
  early for r=0.5 bodies), so the door never throttles below ~2 bodies/step; (iii) push-out
  admits transient overlaps under crowd pressure. This is a known property of naive CA egress
  models without density-dependent speed rules.
- Pre-registered risk clause applied (plan risk register; D27): the claim carried forward is
  the qualitative congestion/flow-limitation pattern (P2), with the quantitative deviation
  reported openly — EXP-B's component verification (T1/T6/T7 PASS) and EXP-D docking carry the
  quantitative adequacy weight instead.

Figures (30 overlaid R(t) curves, T_drain histogram vs band, F̂s per rep, v̂_free distribution)
are rendered in Phase 4 (`harness/figures.py`) per STATUS.
