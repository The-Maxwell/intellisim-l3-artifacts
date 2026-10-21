# EXP-H2 — regeneration (F2) and model-only ablation (A): report

> Protocol: `experiments/37-exph2-regeneration-ablation-protocol.md`, pre-registered in commit `21a1e2d` (2026-09-23 18:51:49Z) before any generation call. Decisions D24–D26. Outcomes computed by `harness/exph2_score.py` → `OUTCOMES.json`, `rubric3.csv`. Numbers of record for the article go to `results/_article/KEY-NUMBERS.md` §10.

## 1. Execution (2026-09-23, 18:52–19:12Z)

| Item | Value |
|---|---|
| Gates | G0 PASS (10 source sims finished, windows 45/44/599/899/97/45/10/85/227/173, original analyses present, o4-mini with `max_tokens`); G2 PASS (A-1, F2-1 completed, no warnings, ≤ $0.06, live A-1 section bodies byte-equal to the pre-registered dry-run bodies, cap echoed by the engine); G3 never triggered |
| Reports | 20/20 completed (10 F2, 10 A), 0 warnings, every call on `o4-mini-2025-04-16` |
| Calls | 4 successful calls per report in both arms; A: 40 HTTP attempts, 0 failed; F2: one inferred interrupted attempt in F2-10 (wall-clock 140,7 s vs 78,4 s summed latency, gap 62,3 s ≈ 60 s provider timeout + retry backoff — the D25 timing pattern); billing shows no extra request, so this attempt was **not billed** (unlike report 2 on 2026-07-24, whose interrupted attempt was billed 100 003 output tokens) |
| Wall-clock per report | A 32,0–74,9 s (mean 46,6 s); F2 47,2–140,7 s (mean 70,2 s; 47,2–77,7 s without F2-10) |
| Cost of record (billing × list price) | **$0.646** (A $0.229, F2 $0.417; F2 ≈ $0.042 per report, replicating the corrected o4-mini ≈ $0.041 of D25). The usage export (written 2026-09-23 19:34Z) equals the recorded usage to the token: 80 requests, 168 039 input, 104 806 output tokens (`COST.md`, D28) |
| Order | pilot A-1, F2-1; then n = 2…10, odd n A→F2, even n F2→A |

## 2. Pass 3 (blind, sheet-only, LLM-assisted)

30 reports (10 original EXP-H, 10 F2, 10 A) converted to one neutral JSON (sections + synthesis + language only), random IDs, seed 4094598122 written before scoring (`scoring-3/SEED.txt`), mapping in `scoring-3/_mapping.json` (never given to scorers). 10 fresh scorer agents × 3 reports; each agent got three different runs, one report of each arm, in shuffled order. Rubric = `RUBRIC-pass2.md` verbatim + the `[expectation]` tagging note (`scoring-3/RUBRIC.md`). Outputs `scoring-3/R01.md … R30.md`.

## 3. Results

### 3.1 Per arm (pass 3)

| Arm | Points / 60 | D | Perfect (6/6) | Factual errors (per report) | Hallucinated phenomena | Recommendations present |
|---|---|---|---|---|---|---|
| Original EXP-H reports | **40** | **0,667** | 0 | 8 (0,8) | 11 | 10/10 |
| F2 — regenerated, full evidence | **44** | **0,733** | 2 | 5 (0,5) | 8 | 10/10 |
| A — model-only (no run data) | **14** | **0,233** | 0 | 11 (1,1) | 14 | 10/10 |

### 3.2 Per run (points of 6, pass 3)

| Run | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | Σ |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Original | 4 | 4 | 5 | 2 | 4 | 2 | 5 | 5 | 5 | 4 | 40 |
| F2 | 3 | 2 | 5 | 3 | 5 | 4 | 6 | 6 | 5 | 5 | 44 |
| A | 1 | 2 | 2 | 0 | 0 | 1 | 0 | 4 | 2 | 2 | 14 |
| Original − A | 3 | 2 | 3 | 2 | 4 | 1 | 5 | 1 | 3 | 2 | 26 |

**Primary outcome:** original > A in **10 of 10** runs (no ties), Σ difference 26 points; one-sided sign test **p = 0,00098** (= 1/1024).

### 3.3 Arm A by pre-registered item class (protocol 37 §4)

| Class | Items | A points / max | 2-point items in A | Original (pass 3) | F2 (pass 3) |
|---|---|---|---|---|---|
| M — metadata/prior can reach 2 | 11 | 8 / 22 | 1 (8P2, burn-through at p = 0,70) | 13 | 15 |
| P — prior can reach 1 | 9 | 6 / 18 | 0 | 16 | 14 |
| D — data-dependent | 10 | **0 / 20** | 0 | 11 | 15 |

### 3.4 Pre-registered predictions

| Prediction (§4 / §6) | Result | Verdict |
|---|---|---|
| D(A) between 5 and 25 of 60, central 12–18 | 14 | met (central) |
| A's points concentrated in M items | M 8, P 6, D 0 | met |
| ≥ 90 % of D-item scores in A equal 0 | 10/10 = 100 % | met |
| A has more hallucinated phenomena than the originals | 14 vs 11 (errors 11 vs 8) | met |
| D(F2) ≥ 0,7 | 0,733 | met |
| Decision rule D(A) ≥ 0,5 ⇒ rubric's 1-point level non-discriminating | D(A) = 0,233 | not triggered |
| Decision rule D(F2) < 0,7 ⇒ criterion 1 not replicated | 0,733 | not triggered |

### 3.5 Agreement

| Pair (30 primary items) | Exact agreement | κ | κ linear |
|---|---|---|---|
| pass 2 vs pass 3, original reports (both blind, sheet-only) | 80 % | **0,646** | 0,685 |
| pass 1 vs pass 3, original reports (pass 1 had the step series) | 56,7 % | 0,244 | 0,305 |
| (reference, `kappa.md`) pass 1 vs pass 2 | 73 % | 0,437 | 0,487 |
| original vs F2, both scored in pass 3 (regeneration) | 60 % | 0,305 | 0,392 |

Per-run F2 − original (pass 3): −1, −2, 0, +1, +1, +2, +1, +1, 0, +1.

### 3.6 Criteria under pass 3

- Criterion 1 (D ≥ 0,7): original **0,667 — not met**; F2 **0,733 — met**. A 0,233.
- Criterion 2 (≤ 1 error per report): original 0,8 met; F2 0,5 met; A 1,1 not met.
- The three scorings of the original reports are now 54/60 (pass 1, with series), 46/60 (pass 2, blind), 40/60 (pass 3, blind): **67–90 %**. Criterion 1 holds in passes 1 and 2, not in pass 3. Protocol 37 §5 fixed pass 3 of the originals as the same-session comparator for A and F2, not as a replacement headline; the value is nevertheless reported, as protocol 37 §6 requires.

## 4. Qualitative observations (from the scorer sheets and the reports; no scores changed)

1. **Without run data the model substitutes priors, and those priors are wrong for these runs.** A-5 and A-6 read the configured step limit as the run's length («ran for the full 300-step time cap», «Симуляція завершилася за 300 кроків»; data: 98 and 46 steps). A-7 (p = 0,50, sub-critical) has the fire burn «впродовж кількох сотень кроків» «через весь ліс» (data: out by step 10, 5,9 % burned) although the run name carried p. A-9 and A-10 give the textbook well-mixed outcome («all agents in the recovered state and herd immunity achieved»; data: final size 0,37 and 0,33). A-4 asserts classic flow saturation at the opening (the engine has no speed–density coupling). Only 1 of A's 11 counted errors is the statement that no time series was recorded (A-1).
2. **Abstention on the egress time.** A-1 and A-2 state that no egress time can be given; neither falls back to the SFPE ≈ 110 s. The 44–46-step egress in the full reports (and in F2-1/F2-2: P3 = 2 and 1) is therefore taken from the data.
3. **Metadata-driven content in A.** A-4 reproduces the model description's «density measured in a 4x2 m rectangle upstream» as an observation («у квадраті 12×6 клітин (4×2 м) перед отвором щільність постійно зростала…») — a hallucination sourced from M. A-3 credits upstream congestion only from the description («design-intent» quotes, tagged `[expectation]`: 1 + 1 points).
4. **Report 4 on regeneration.** F2-4 again derives a within-run peak (97 agents in steps 30–45, ≈ 6,5 per step) and relates the later decline to falling density («Зниження потоку корелює зі зменшенням щільності біля отвору»), a weaker form of the original's density–flow reading; pass 3 scored it 3/6 (original 2/6).
5. **The binary usefulness criterion does not discriminate:** model-only reports contain actionable recommendations in 10/10 cases.
6. **Robustness note:** in F2-6 the model placed its recommendations inside `conclusions` as a malformed JSON fragment (the `recommendations` field is empty); the engine accepted the object because it parsed.

## 5. Reading (for the article; wording decided with the author)

- The deterministic evidence pack carries the result: under the same blind scoring, the full reports earn 67–73 % of the points (original 40, regenerated 44 of 60) against 23 % without run data (14 of 60), in 10 of 10 runs; no data-dependent item scores without data. The rubric therefore discriminates data-grounded from prior-driven reports; its floor from priors + metadata is ≈ 23 %, almost all of it on qualitative or metadata-stated items.
- The metadata leak (D26) is bounded: the model-only arm reaches 8 of 22 points on the metadata/prior-attainable items and 0 on data-dependent items.
- Regeneration changes item scores substantially (60 % exact agreement, κ 0,31) but not the aggregate (40 vs 44 of 60): report-level nondeterminism is of the same order as scorer disagreement.
- Scorer disagreement is concentrated between the series-informed pass 1 and the blind passes: the two blind passes agree substantially (κ 0,65), each agrees only fairly/moderately with pass 1 (0,24; 0,44). This quantifies the information-asymmetry reading of D17/D18.
- The criterion D ≥ 0,7 is sensitive to the scorer: 2 of 3 scorings of the originals meet it; the regenerated reports meet it under pass 3.
