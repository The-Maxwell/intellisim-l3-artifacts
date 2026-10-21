# POOLED — post-hoc, descriptive statistics over the three scoring passes (v7, D32; `harness/exph_pooled.py`)

> **Not pre-registered.** Protocol 36 defines pass 1 as the scoring of record; protocol 37 §6 defines the primary outcome
> (original > no-data per run, one-sided sign test). Everything below is a post-hoc description over the three scorings
> of the primary series (reports 1–10) and never replaces the pre-registered verdicts (KEY-NUMBERS §2, §8, §10).

## 1. Per pass (reports 1–10)

| Pass | Points / 60 | D | Errors (mean / report) | Hallucinated phenomena | D 95 % CI (bootstrap by report) |
|---|---|---|---|---|---|
| 1 | 54 | 0.900 | 7 (0.7) | 1 | [0.800; 0.983] |
| 2 | 46 | 0.767 | 11 (1.1) | 9 | [0.650; 0.850] |
| 3 | 40 | 0.667 | 8 (0.8) | 11 | [0.550; 0.767] |

## 2. Pooled over passes I–III

| Quantity | Value | 95 % CI (bootstrap by report) | Print form (UA) |
|---|---|---|---|
| D pooled = Σ points / 180 | 140/180 = 0.7778 | [0.672; 0.867] | «77,8 % (95 % ДІ 0,67–0,87)» |
| Mean errors per report-scoring = Σ errors / 30 | 26/30 = 0.8667 | [0.600; 1.200] | «0,87 помилки на звіт» |
| Mean hallucinated phenomena per report-scoring | 21/30 = 0.70 | — | — |
| One-sided sign test, 10 wins of 10 | p = 0,5^10 = 0.0009765625 = 1/1024 | — | «p = 2^−10 ≈ 0,001» |

## 3. Bootstrap method

- Resampling unit = the report (n = 10): draw 10 reports with replacement, B = 10000, `numpy.random.default_rng(20261012)`,
  `rng.integers(0, 10, size=(B, 10))`; the same drawn reports are used for all three passes, so the pooled statistic keeps
  the within-report correlation across passes. Percentile interval (2,5 / 97,5 %, `numpy.percentile`, linear interpolation).
- D moves in steps of 1/60 per pass, so the bounds are coarse; items inside a report are not independent (3 per report),
  which the report-level resampling respects. Resampling the 30 report-scorings independently would give a narrower,
  anti-conservative interval and is not used.

## 4. κ labels (Landis & Koch 1977 bands: 0,21–0,40 fair, 0,41–0,60 moderate, 0,61–0,80 substantial)

| Pair | κ | Label | UA |
|---|---|---|---|
| pass 1 – pass 2 (30 items) | 0.44 | moderate | помірна |
| pass 2 – pass 3 (30 items) | 0.65 | substantial | суттєва |
| pass 1 – pass 3 (30 items) | 0.24 | fair | слабка |
| original – F2, pass 3 (30 items) | 0.30 | fair | слабка |

Sign test of record (OUTCOMES.json primary.sign_test): 10 positive, 0 negative, 0 ties.
