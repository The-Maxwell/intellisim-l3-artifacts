# experiments/

The public record of the Level-3 evaluation behind the article — the pre-registered protocol and
its addendum, the measurement scripts, and every result file the article's numbers come from.

## Layout

- `protocols/36-analysis-eval-protocol.md` — EXP-H protocol (design, rubric, acceptance
  criteria), pre-registered before the first analysis; the ground-truth sheets and the run list
  were committed first (blinding gate).
- `protocols/37-exph2-regeneration-ablation-protocol.md` — EXP-H2 addendum (regeneration arm F2,
  no-data ablation arm A, blind pass III), committed with frozen request hashes before any
  generation call.
- `harness/` — Python scripts: rubric parsing (`exph_rescore.py`), Cohen's κ (`kappa.py`),
  objective data check (`exph_datacheck.py`), pooled statistics (`exph_pooled.py`), article tables
  (`exph_tables.py`) and figures (`l3_figures.py`, `figstyle_uk.py`), cost of record (`exph_cost.py`,
  `exph2_cost.py`), and the EXP-H2 generation pipeline (`exph2_preflight.py`, `exph2_ablate.py`,
  `exph2_regen.py`, `exph2_run.py`, `exph2_blind.py`, `exph2_score.py`, `exph2_prereg.py`,
  `exph2_common.py`). Runnability notes: root `README.md` → «Reproducibility notes».
- `results/exp-h/` — EXP-H (2026-07-24): `manifest.json` (per report: deployment id, token
  totals, summed latency of the four calls), `cost.json`, `REPORT.md`, `ground-truth/sheet-1..10.md`, `report-1..12/`
  (`request.json`, `report.json`, `analysis.pdf`, `scoring.md` = pass I), `PROVENANCE.md` (SHA-256
  of every file copied from the campaign repository).
- `results/exp-h-rescoring/` — pass II (blind, sheet-only): `PROTOCOL.md`, `RUBRIC-pass2.md`,
  `scoring-2/report-N.md`, `rubric.csv` (passes I and II side by side), `kappa.md`, `ANALYSIS.md`;
  objective data check `DATA-CHECK.md`, `data-check.csv`.
- `results/exp-h2/` — EXP-H2 (2026-09-23): `manifest.json`, `preflight.json`, `dryrun/`
  (pre-registered request bodies and hashes), `f2-1..10/` (regenerated reports), `a-1..10/`
  (no-data reports with raw request/response JSON), `scoring-3/` (pass III, blinded set with
  `_mapping.json` and `SEED.txt`), `rubric3.csv`, `OUTCOMES.json`, `COST.md`, `REPORT.md`,
  `battery.log`.
- `results/source-runs/` — the 31 exported files of the ten analysed runs (step series,
  simulation definitions, metrics, and the `REPORT.md` files of the earlier campaign's experiments,
  not of this article) from which the ground truth was measured; `MANIFEST.md` maps
  report → run → files → measured values.
- `results/_article/` — `KEY-NUMBERS.md` (number registry with provenance for every value the
  article prints, including the corrections to the original `REPORT.md`), `POOLED.md`,
  `tables/TABLES.md`, `figures/l3-f1..f4.{pdf,png}` (the article uses f1 and f2).

## How to read a report folder

1. `request.json` — the exact request the harness sent to `POST /simulations/:id/analyses`
   (window, selection, model, hint).
2. `../manifest.json` — the record of the generation: deployment id, token totals and summed
   latency of the four calls, status, source simulation id.
3. `report.json` / `analysis.pdf` — the generated report (JSON as stored by the core; PDF as
   served from GridFS).
4. `scoring.md` — pass I scoring against `../ground-truth/sheet-N.md` (0/1/2 per phenomenon,
   factual errors, hallucinated phenomena, usefulness quote).
5. `../../exp-h-rescoring/scoring-2/report-N.md` and `../../exp-h2/scoring-3/` — passes II and III
   of the same report (pass III sheets are blinded; `_mapping.json` resolves them).

## Provenance

Copied verbatim from the authors' campaign repository `intellisim.experiments.l3` at commit
`44bca1b` (committed 2026-10-20, copied 2026-10-21); the EXP-H inputs in turn are byte-identical
to the original campaign files, hash-listed in `results/exp-h/PROVENANCE.md`. Documents cite
paths relative to that repository's root (`results/…`, `experiments/NN-….md`, `harness/…`); here
they live under `experiments/results/…`, `experiments/protocols/…`, `experiments/harness/…`.

Three cells of `results/_article/KEY-NUMBERS.md` §6 were edited for publication: the table-header
path `../intellisim.core` → `intellisim.core`, a reference to a private workspace file removed
from the entry-point provenance cell, and the provenance of the «Not present (honest scope)» row
(a private design document and an internal evidence assessment) replaced by a pointer to the
code. Nothing else was changed.

`PROVENANCE.md` resolves as follows in this archive: 60 of its 67 rows plus all 31 source-run rows
verify byte-for-byte; the five `results/_billing/*` rows refer to files that are intentionally
not included; `results/exp-h/REPORT.md` and `protocols/36-analysis-eval-protocol.md` carry a
provenance header added after hashing, as the file's own first paragraph states, so their hashes
are those of the header-less originals. The frozen-artefact table of protocol 37 §8 verifies in
full.

Known stale paths in the copied documents: `results/_billing/…` and
`results/_article/EVIDENCE-ASSESSMENT.md` (not included — the OpenAI usage exports contain
account identifiers; the evidence assessment is an internal working note); `article/…`,
`DECISIONS.md`, `STATUS.md`, `PLAN.md`, `RUNBOOK.md` (the authors' private manuscript and
planning files — «D-numbers» are entries of that private decision log, and commit ids such as
`23ff33d`, `21a1e2d`, `a9694df` refer to the private campaign repositories); the links in
protocol 36 to protocols 32, 33, 35, 41, 51 and to `ARTIFACTS.md` of the earlier campaign, which
are public in
[`intellisim-experiment-artifacts`](https://github.com/The-Maxwell/intellisim-experiment-artifacts)
under `experiments/protocols/` and `experiments/ARTIFACTS.md`.

All scoring passes were LLM-assisted (pass I sheets: `scorer=Claude (author)`; passes II and
III: `scorer=pass-2-blind` / `pass-3-blind`, see `results/exp-h-rescoring/PROTOCOL.md` and
`results/exp-h2/REPORT.md` §2), as the article states; the data check is rater-free.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE), except the third-party
files listed in [`../THIRD-PARTY-NOTICES.md`](../THIRD-PARTY-NOTICES.md).
