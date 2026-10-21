# Source runs analysed by EXP-H — what the Level-3 reports were generated from, and where the ground truth was measured

> Task 2b, 2026-09-08. The ten simulations analysed in EXP-H were produced and measured by the authors' earlier benchmark experiments (EXP-B/C/E/F/G, campaign `intellisim.experiments`, 2026-07). Their exports are the *ground truth* the ten sheets score against. This folder holds the byte-identical subset of that campaign's `results/` that the sheets cite (same relative paths, sha256 in `../exp-h/PROVENANCE.md` §Source-run subset), plus each run's definition export and the source experiments' REPORT.md for context. The public L1+L2 bundle (`intellisim-experiment-artifacts`) does **not** contain the EXP-B/EXP-C runs used here (it ships re-run batteries `exp-b-v2`/`exp-c-v2` with other simulation IDs), so this subset is the only public-able copy — it goes into `intellisim-l3-artifacts` (Task 12).

## Report → run → files → measured ground truth

| Rep. | Source experiment / run | Sim ID | Files in this folder | Measured values used as ground truth (3 phenomena) |
|---|---|---|---|---|
| 1 (en, o4-mini) · 11 (en, o3) | **EXP-C** room evacuation, 100 agents, one door; median-T_drain rep (rep3, seed 103) | `6a5bbcaac44d001ef3ac207c` | `exp-c/raw/…207c-rep3-steps.csv`, `…207c-rep3-simulation.json`, `exp-c/metrics.json`, `exp-c/REPORT.md` | door queue, throughput ≈ 2.2 agents/step (2.0–2.4); near-linear discharge (98 @ 43, 100 @ 45); T_total = 45 steps = 13.5 s @ 0.3 s/step (ensemble 44–47 steps; ≈ 6.6× faster than SFPE ≈ 110 s — engine-class deviation) |
| 2 (uk, o4-mini) · 12 (uk, o3) | **EXP-C**, fastest-T_drain rep (rep18, seed 118) | `6a5bbcc1c44d001ef3acc10e` | `exp-c/raw/…c10e-rep18-steps.csv`, `…c10e-rep18-simulation.json`, `exp-c/metrics.json` | throughput ≈ 2.3 agents/step (2.0–2.5); near-linear (98 @ 42, 100 @ 44); T_total = 44 steps = 13.2 s |
| 3 (en) | **EXP-B** RiMEA Test 12 bottleneck, 150 agents, 2 rooms + corridor (rep1, seed 1201) | `6a5bc249c44d001ef3af684d` | `exp-b/test-12/raw/…684d-rep1-steps.csv`, `…684d-rep1-simulation.json`, `exp-b/test-12/REPORT.md` | congestion upstream: zone-1 peak 37.2 vs zone-2 12.7 (ratio 2.9); zone-1 ≥ 10 for ≈ 34.5 steps, 3 agents wedge; 147/150 out by ≈ step 98, 3 stragglers to max_steps 599 |
| 4 (uk) | **EXP-B** RiMEA Test 4 wide opening (10 cells / 4 m), N = 300 (rep1, seed 401) | `6a5bc463c44d001ef3b8f824` | `exp-b/test-4/N300/raw/…f824-rep1-steps.csv`, `…f824-rep1-simulation.json`, `exp-b/test-4/fundamental-diagram.json`, `exp-b/test-4/REPORT.md` | 297/300 out by step 83 ⇒ ≈ 3.6 agents/step (3.3–3.9); specific flow rises with density (RiMEA rising branch within 10–15 % to ≈ 1.5 P/m²); no saturation beyond ≈ 2 P/m² (ensemble property over N = 50/150/300 — the single-run boundary case) |
| 5 (en) | **EXP-E** `archetype_decision` (LLM gpt-4.1-mini decides per archetype), rep301, seed 301 | `6a6380584ccaa6983ac7ea1c` | `exp-e/archetype_decision/run-301/{metrics.json, run.json, export.csv.gz}`, `exp-e/stats.json`, `exp-e/REPORT.md` | T_total 98 steps (29.4 s) vs rules-only 46; per-agent egress 10.8 ± 6.4 s vs 6.7 ± 3.3 s; labels 441 proceed / 178 follow_crowd / 55 wait_briefly / 40 seek_alternate (714 events); condition_met |
| 6 (uk) | **EXP-E** `individual_decision` (LLM decides per agent), rep301, seed 301 | `6a63810f4ccaa6983ac81367` | `exp-e/individual_decision/run-301/{metrics.json, run.json, export.csv.gz}`, `exp-e/stats.json` | T_total 46 steps = baseline; egress 6.76 ± 3.25 s ≈ 6.73 ± 3.28 s; labels degenerate: 494/494 proceed_to_exit; condition_met |
| 7 (uk) | **EXP-F** wildfire (NetLogo Fire docking), tree density p = 0.50 (sub-critical, p_c ≈ 0.59), rep1 | `6a610ff2ff82d466bf2db995` | `exp-f/intellisim/sweep-61/raw/…b995-p0.50-rep1-steps.csv.gz`, `exp-f/intellisim/sweep-61/metrics.json`, `exp-f/REPORT.md` | fire out by step 10; burned fraction 0.0594 (sweep 0.098 ± 0.044); percolated = False (sweep 0.0) |
| 8 (en) | **EXP-F** wildfire, p = 0.70 (super-critical), rep1 | `6a634dbf4ccaa6983a640151` | `exp-f/intellisim/sweep-61/raw/…0151-p0.70-rep1-steps.csv.gz`, `exp-f/intellisim/sweep-61/metrics.json` | burned fraction 0.9925 (sweep 0.976 ± 0.008); percolated = True (sweep 1.0); run length 85 steps |
| 9 (en) | **EXP-G** spatial SIR epidemic (epiDEM docking), 200 agents, medium transmissibility β = 0.18, rep1 (seed 8501) | `6a636be64ccaa6983aafedd5` | `exp-g/intellisim/manifest.json`, `exp-g/intellisim/point-med/curves.jsonl`, `exp-g/intellisim/point-med/raw/med-rep1-steps.csv.gz`, `exp-g/stats.json`, `exp-g/REPORT.md` | single peak I = 22 at step 60 (run 228 steps); final size 74/200 = 0.37 (well-mixed analytic 0.822 = upper bound); S↓ R↑ monotone |
| 10 (uk) | **EXP-G**, same point, rep8 (seed 8508) | `6a636c0c4ccaa6983ab1976a` | `exp-g/intellisim/manifest.json`, `exp-g/intellisim/point-med/raw/med-rep8-steps.csv.gz`, `exp-g/stats.json` | single peak I = 21 at step 51 (run 174 steps); final size 66/200 = 0.33; S↓ R↑ monotone |

Values are transcribed from `../exp-h/ground-truth/sheet-N.md` (pre-registered, sibling commit `23ff33d`); the sheet is authoritative, this table is a reading aid and the seed for the article's «source runs» table.

## What each experiment family was (one line each, for the article's methods)

| Family | Benchmark anchor | Engine mode | Why it is a good ground truth |
|---|---|---|---|
| EXP-C room evacuation | SFPE hydraulic egress model (measured deviation documented) | rules only | throughput and egress time are directly measurable from `evacuated_count`; the analysis must report the *measured* time, not the textbook one |
| EXP-B RiMEA tests 4 / 12 | RiMEA guideline test cases (fundamental diagram; bottleneck) | rules only | zone occupancy and flow are exported per step; T4 truth is an ensemble property → boundary case for single-run analysis |
| EXP-E strategy study | EXP-C rules-only run as reference | Level-2 LLM decisions (archetype vs individual) | decision-label counts are exact integers; slower/spread vs baseline is a measured contrast |
| EXP-F wildfire | NetLogo Fire model docking; percolation threshold p_c ≈ 0.59 | rules only | regime (sub/super-critical), burned fraction and percolation are binary/near-exact |
| EXP-G epidemic | NetLogo epiDEM docking; SIR final-size relation | rules only | peak height/timing and final size are exact integers; SIR monotonicity is an invariant |

## How the analysed runs relate to the article

- Methods: state that the ten runs come from the authors' earlier benchmark experiments, that their key quantities were measured from exported step series *before* any analysis and independently of the analysis model, and give the table above in compact form (scenario, size, measured values). Do not cite the unpublished L1+L2 article (D10); describe the runs directly.
- Data availability: the L3 artifacts repository carries this subset, the sheets, the 12 reports (JSON + PDF) and both scoring passes.
- Do not claim the runs are «validated against SFPE/RiMEA/NetLogo» beyond what the source REPORT.md files say; EXP-C deviates 6.6× from SFPE by design of the engine (no speed–density coupling) and the sheets score against the data, not the anchor.

## Not copied (and why)

Full experiment folders (EXP-B 107 MB / 450 files, EXP-F 556 MB / 594 files, EXP-C 6 MB, EXP-E 2 MB, EXP-G 16 MB), other replications, NetLogo baseline outputs, figures. They are not referenced by the sheets. If a referee needs them, they are in the campaign repo; the public L1+L2 bundle carries the re-run batteries and the EXP-E/F/G data.
