# Reading the two scoring passes — what the disagreement means (Task 3, 2026-09-08)

> Companion to the generated `kappa.md`. Nothing here changes a score; both passes are reported as data (D17). Pass 1 = original author scoring 2026-07-24 (`results/exp-h/report-N/scoring.md`); pass 2 = blind re-scoring 2026-09-08 (`scoring-2/report-N.md`, protocol `PROTOCOL.md`).

## Headline

| Quantity (primary reports 1–10) | Pass 1 | Pass 2 | Both passes |
|---|---|---|---|
| Phenomenon points | 54/60 (90.0 %) | 46/60 (76.7 %) | criterion 1 (≥ 70 %) **passes in both** |
| Factual errors, mean / report | 7 (0.7) | 11 (1.1) | criterion 2 (≤ 1.0) **passes in pass 1 only** |
| Hallucinated phenomena | 1 | 9 | — |
| Usefulness | 10/10 | 10/10 | identical |
| Cohen's κ on 30 items (36 items) | — | — | 0.437 (0.422), linear-weighted 0.487 (0.472): **moderate**; exact agreement 73 % (75 %) |

## Structure of the disagreement (from `kappa.md`)

1. **Phenomenon scores: one-directional.** All 9 disagreements out of 36 items are pass-1 = 2 → pass-2 = 1. No 0↔2 reversal, no case where pass 2 was more generous. Pass 2 applied «quantitatively correct within the data-supported range» more literally: where a report identified the phenomenon but framed the run as phased (surge / plateau / phase transition / secondary waves), or quoted a rate outside the sheet's range (≈ 3 agents/step vs 2.0–2.5), pass 2 gave 1; pass 1 gave 2 and logged the framing as a note. The sum difference (8 points) is entirely this shift.
2. **Error counts: poorly reproducible (2/12 exact matches).** Two sources:
   - *Regime-structure claims.* The reports' `emergent_phenomena` and `causal_links` fields regularly assert within-run structure («throughput surge steps 14–24», «phase transition to a sparse regime», «secondary waves», «plateau 3.1/step»). The sheets state the run-level truth (near-linear discharge; single hump) without a step-by-step series. Pass 2 counted such claims as contradictions or hallucinations (11 of its 14 errors and 9 of its 11 hallucinations are of this type); pass 1, which had the raw step CSVs at hand, treated most of them as «internal wobble, not a data contradiction». **The rubric does not settle this case** — it is the main reproducibility gap.
   - *Genuine catches by pass 2 that pass 1 missed.* Report 4: «середній потік ≈ 0,66 агента/крок … до кроку 449» (data: 297/300 out by step 83 ≈ 3.6/step). Report 9: «snapshots show only susceptible agents» at step 227 (74 recovered are present; R is monotone). Report 10: «66 поточних піддатливих» at step 86 (66 is the final recovered count; S ends at 134). Report 8: trees «x = 3 to x = 54» while ash exists at x = 60. Report 3: queue placed «at the final exit» (data: upstream Room-1 door). These are checkable against the sheet alone and are plausibly real errors.
   - *Pass-1 errors not re-found by pass 2.* Report 6 «single-cell door» (pass 2: 0 errors); report 7 «4.4 % vs 138 ash» inconsistency (pass 2 kept only the direction error).
3. **Model contrast is not robust.** Pass 1: o3 12/12 & 0 errors vs o4-mini 11/12 & 1 error on the same runs. Pass 2: o3 11/12 & 3 errors vs o4-mini 9/12 & 3 errors. The direction (o3 ≥ o4-mini on points) holds; the «zero errors» claim does not. Report 12's correct unit inference (13,5 с) is confirmed by pass 2 (P3 = 2 in both).
4. **Language contrast is not robust either.** Pass 1 en 29 vs uk 25 (errors 1 vs 6); pass 2 en 26 vs uk 20 (errors 5 vs 6). uk stays lower on points in both; the error asymmetry disappears in pass 2.
5. **Robust across passes:** report 4 is the weakest (3/6 → 2/6, 2 errors, 1 hallucination in both); the EXP-C egress is reported as measured (44–46 steps) in all four EXP-C reports in both passes (P3 = 2 for reports 1, 11, 12 in both; report 2 = 1 in both for the «≈ 57 с» slip); exact reproductions (441/178/40/55; peak 22 @ 60; 37 %; 97.8 %) are scored 2 in both; usefulness 12/12 in both.

## Asymmetry of the two passes (must be stated with κ)

Pass 1 had the exported step series; pass 2 had only the sheet (measured values + data-supported ranges). A regime claim that is true of the series but not derivable from the sheet is therefore an «error» for pass 2 and a «note» for pass 1. Part of the 7-vs-14 gap is this information asymmetry, not scorer disagreement. Whether the disputed claims (e.g. «33 persons exit in steps 14–24») hold in the CSVs is machine-checkable from `results/source-runs/**` — **not done in Task 3** (it would be a third, adjudicating pass; listed as an optional arm below).

## What the article should say

- Report both passes and κ; headline the faithfulness result as **77–90 % of pre-registered phenomena recovered**, criterion 1 met in both passes; error rate **0.7–1.1 per report**, meeting the pre-registered ≤ 1 threshold in one of two passes; hallucinations 1–9 depending on how within-run regime claims are treated.
- State that inter-pass agreement on phenomenon detection is moderate (κ ≈ 0.44) and that error/hallucination counts are the least reproducible part of the rubric — this is a finding about evaluating LLM analyses, not a footnote.
- Keep the robust findings (report 4 boundary case, reads-data-not-prior, exact reproductions, usefulness) and drop or soften the fragile ones (o3 «zero errors»; en/uk error asymmetry).
- Do not claim a human inter-rater study; both passes are LLM-assisted (D12).

## Improvement points added by this task (per D3, not executed)

| Arm | What | Value |
|---|---|---|
| Objective data-check of disputed claims | for every disputed regime claim, compute the quantity from the step series (`results/source-runs/**`) and mark it true/false | converts part of the inter-pass gap into fact; rater-free |
| Rubric clarification | add to protocol §2: how to score within-run regime claims when the sheet states only run-level truth (either supply the series to the scorer or define them as out of scope) | removes the main reproducibility gap for future runs |
| Equal information for both scorers | give the second scorer the same step series as the first | makes κ interpretable as pure rater disagreement |
| Human second scorer | as in `EVIDENCE-ASSESSMENT.md` §5 | closes the LLM-scores-LLM vulnerability |

## Task 3b (2026-09-09, D18) — the disputed claims checked against the step series

`harness/exph_datacheck.py` recomputes every statement the two passes disagreed on (or that pass 2 counted without access to the series) from `results/source-runs/**` and writes `DATA-CHECK.md` / `data-check.csv`. 50 statements; the 25 that pass 2 counted as errors or hallucinations map one-to-one onto its 14 + 11. Rule: TRUE = figure reproduces within rounding; PARTLY = qualitative statement holds but a number is off (or the reverse); FALSE = contradicted; a regime change needs a ≥ 30 % rate difference between the two windows, a secondary SIR wave a ≥ 3× rebound from a trough to ≥ 5 infected. **No score is changed.**

| Set | TRUE | PARTLY | FALSE | UNCHECK | n |
|---|---|---|---|---|---|
| all checked statements | 19 | 21 | 9 | 1 | 50 |
| pass-2 errors + hallucinations | 6 | 10 | 8 | 1 | 25 |
| pass-1 errors + hallucinations (checkable ones) | 0 | 3 | 2 | 0 | 5 |

What the series settle:

1. **The EXP-C «regime» claims are largely true of the runs.** rep3: exits per step ramp 1.6 (steps 1–13) → 3.3 (steps 15–24, exactly 33 exits) → 2.1 (steps 25–45); «65 exits in steps 10–30, 3.1/step, peak 5 at step 24» reproduces exactly; the late slowdown is real (3.1 → 1.4 per step, −54 %) though «≈ 1.1» overstates it. rep18: «≈ 3 per step, steps 9–30» = 3.09; «plateau 4–5» — eight steps with ≥ 4 exits, max 5; the outflow halves after step 34 (3.07 → 1.50, ratio 0.49). Only report 1's «phase transition at 50 %» fails the 30 % rule (2.33 → 2.10 per step). The sheet's «near-linear ≈ 2.2/step» is also true (R² 0.987 / 0.991): the two descriptions differ in temporal resolution, not in fact. Pass 2, seeing only the run-level range, counted six true statements as errors/hallucinations (1a, 11a, 2a, 12b, 12c, 9a) — this is the information-asymmetry part of the gap, now quantified.
2. **Pass 2's genuine catches are confirmed**: queue placed at the final exit (report 3; stragglers sit at x = 24–27, the Room-1 door), «0.66 agents/step to step 449» (report 4; 297 out by step 83), trees «x = 3 to 54» (report 8; x range 0–60), «only susceptible agents at steps 113/227» (report 9; R = 46 / 74), «66 susceptible at step 86» (report 10; S = 154). Pass 1, which had the series, missed all five. Two errors both passes counted are confirmed (report 5 straggler; report 7 fire direction — mean x of burning cells rises 0.5 → 4.6).
3. **Report 9's «secondary waves» are real**: after the peak (22 at step 60) I falls to 1 at step 138 and rises to 9 at step 177 (9× rebound). Report 10's rebound 3 → 10 (steps 84–104) is real too, but its «steps 56–64» window shows no burst (PARTLY).
4. **Report 4's «maximum ≈ 6.5/step at ≈ 160 remaining»** is numerically true of the time series (best 5-step window 7.2/step with 172 remaining; band rates 4.2 / 6.8 / 2.1 per step for > 200 / 200–100 / < 100 remaining). The error is the reading of an arrival-ramp-then-depletion profile as a density–flow law — a category error, not a fabricated number. The boundary case stands, sharpened.
5. **Mechanism claims stay undecidable** (update ordering, «jam caused by opening width», herd-immunity threshold); the three stragglers of report 4 do sit in the opening (x 39–42, y 16), static from step 93 to 899.

Descriptive lower bound (not a rubric count): among the checked statements 9 are FALSE and one further error (report 2, «≈ 57 с») is undisputed — 10 data-contradicted statements over the 10 primary reports, with 21 PARTLY items whose treatment decides whether criterion 2 (≤ 1.0/report) holds. The objective check therefore neither rescues nor sinks criterion 2; it shows the count sits at the threshold and depends on the rubric's resolution rule.

**Consequence for the article (D18):** keep D17 (both passes, range, κ); add one paragraph: the disputed statements were recomputed from the exports; about a quarter of what the sheet-only pass counted as errors are true finer-grained descriptions of the series, about a third are confirmed errors, the rest are numerically imprecise; the rubric's «data-supported range» must state its temporal resolution. This turns the «asymmetry» caveat into a measured result and gives the protocol-clarification arm its exact wording.
