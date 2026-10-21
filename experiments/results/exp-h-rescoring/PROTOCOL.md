# EXP-H second scoring pass — blind protocol (Task 3, D2)

> Executed 2026-09-08. Purpose: the pre-registered protocol (`experiments/36-analysis-eval-protocol.md` §2) foresees an «optional inter-rater check: a second scorer scores independently; report Cohen's κ on the 0/1/2 items». Pass 1 (2026-07-24) was scored by the author with LLM assistance; this pass is a second, independent, blind application of the same rubric. Both passes are LLM-assisted (D12): κ therefore measures the **reproducibility of the rubric's application**, not human validity. Original scores are never changed; both passes are reported.

## Inputs the scorer saw (and nothing else)

For each report N = 1…12 a scratch folder contained exactly:

| File | Content | Derived from |
|---|---|---|
| `RUBRIC.md` | protocol §2 verbatim + three operational clarifications (scope = everything under `report` in the JSON; approximate numbers inside the data-supported range are correct; imprecise-but-not-contradicting = 1, contradicting = 0 + error; internal wobbles that do not contradict the sheet are notes; count each distinct erroneous claim once) | protocol §2 |
| `report-N/sheet.md` | the pre-registered ground-truth sheet **without** its original «Scoring» section (which named the pass-1 scorer); for N = 11, 12 the sheet of report 1/2 with a header note that the run is the same and the model differs | `results/exp-h/ground-truth/sheet-N.md` |
| `report-N/report.json` | the archived analysis with `token_usage`, `file`, `created_by`, `requests` removed | `results/exp-h/report-N/report.json` |
| `report-N/TEMPLATE.md` | output skeleton in the pass-1 layout (Meta line pre-filled from `manifest.json` with `scorer=pass-2-blind`) so `harness/exph_rescore.py` parses both passes identically | `results/exp-h/manifest.json` |

The scorer did **not** have access to `report-N/scoring.md`, `results/exp-h/REPORT.md`, `rubric.csv`, `KEY-NUMBERS.md`, the sibling repo, or the web. PDFs were not provided (the PDF is rendered from the same JSON).

## Execution

- Four fresh scoring agents (no shared context with the session that holds pass-1 results), three reports each: {1,2,3}, {4,5,6}, {7,8,9}, {10,11,12}. Each agent was instructed to read only the listed files, read the whole report before scoring, quote verbatim, and be conservative and consistent about errors and hallucinations.
- Outputs `report-N/pass2.md` were copied unchanged to `results/exp-h-rescoring/scoring-2/report-N.md`.
- `python harness/exph_rescore.py --pass2` appends pass-2 rows to `rubric.csv`; `python harness/kappa.py` writes `kappa.md`.

## What is reported (`kappa.md`)

- Cohen's κ (unweighted) and linear-weighted κ over the 36 phenomenon items (12 reports × 3), with % exact agreement and the confusion matrix; the same for the 30 primary items.
- Agreement on per-report factual-error counts and hallucination counts (exact match rate, mean absolute difference, totals per pass).
- Pass-2 aggregates in the same form as pass 1 (points, errors, hallucinations, usefulness, en/uk, o4-mini/o3) and whether the pre-registered criteria hold under pass 2 alone.
- A disagreement log listing every item where the passes differ, with both quotes. Disagreements are data; nothing is adjudicated.

## Known limitations of this pass

Same rubric, same sheets, LLM-assisted like pass 1 → not a human inter-rater study (improvement point in `results/_article/EVIDENCE-ASSESSMENT.md` §5). The scorer saw the JSON, not the PDF, so PDF-rendering issues were out of scope (pass 1 found none).
