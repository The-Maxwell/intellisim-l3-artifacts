# IntelliSim — Level-3 Analysis Artifacts

Public companion archive for the article **«LLM-аналіз результатів агентно-орієнтованого
моделювання: архітектура та оцінювання вірності за еталоном»** (*LLM-based analysis of agent-based
modeling results: architecture and ground-truth faithfulness assessment*) — Pryshchepa V., Zadorozhnii A.,
«Управління розвитком складних систем» (Management of Development of Complex Systems), Kyiv
National University of Construction and Architecture, 2026, issue 68; to appear, DOI to be added.

IntelliSim is a meta-system that turns natural-language descriptions into runnable agent-based
models, executes them with a configurable depth of LLM involvement, and — at its third LLM
integration level, the subject of this article — lets a reasoning model analyse a finished run
from a deterministic evidence pack and write a report whose claims can be checked against the
run's exported data. This repository contains the complete source code of the system at the
exact version named in the article (0.1.2, Level 3 included) and every experiment material the
article's data-availability statement promises: the pre-registered protocol and its addendum,
the ground-truth sheets, the generated reports with their generation manifests, the three scoring
passes, the objective data check, the experiment code, and the subset of source-run exports the
ground truth was measured from.

## Contents

| Folder | Version | Tech | Purpose |
|---|---|---|---|
| `intellisim.core/` | 0.1.2 | Go 1.26, Echo v5, MongoDB, gRPC | Core backend: REST/SSE/WebSocket API, simulation engine (6 LLM execution strategies), rule engine, model generation, LLM bridge, **Level-3 results analysis** (`modules/simulations/analysis_*.go`) |
| `intellisim.front/` | 0.1.2 | Vue 3, TypeScript, PrimeVue, PixiJS | Web UI: chat-based model creation, model editor, live simulation canvas, widget dashboard, analysis view |
| `intellisim.llm.provider.go.sdk/` | 0.1.1 | Go, gRPC, Protobuf | Public SDK for LLM providers; owns the `llmbridge.v1` proto contract |
| `intellisim.openai.llm.provider/` | 0.1.1 | Go | OpenAI adapter microservice (gRPC client of the core) |
| `intellisim.auth.mock/` | — | Node.js | Dev-only OIDC provider (auto-approving; issues real JWTs) |
| `experiments/` | — | Python, Markdown, JSON | The Level-3 evaluation (EXP-H, EXP-H2): protocols, harness, reports, scoring, data check, source-run exports — see [`experiments/README.md`](experiments/README.md) |

The five code folders are byte-identical to the code published with the authors' previous
article in the repository
[`intellisim-experiment-artifacts`](https://github.com/The-Maxwell/intellisim-experiment-artifacts)
(tag `v0.1.2`): the git tree hash of each folder is the same in both repositories. The Level-3 files `modules/simulations/analysis_*.go` did not change between the
July 2026 series (EXP-H) and the September 2026 series (EXP-H2).

Architecture in one paragraph: the frontend talks to the core over REST + SSE + WebSocket
(port 8080). The core persists to MongoDB and exposes a gRPC bridge (port 50051) that LLM
provider processes connect to; the OpenAI provider translates bridge invocations into OpenAI
API calls. Authentication is OIDC: in development the mock server (port 9090) issues tokens.
The frontend dev server runs on port 3000.

**Level-3 entry points.** `POST /simulations/:id/analyses` starts an analysis asynchronously
(202); progress and completion arrive over SSE (`analysis.progress/completed/failed`); the
report is served by `GET /simulations/:id/analyses/:aid` (JSON) and
`GET /simulations/:id/analyses/:aid/file` (PDF stored in the MongoDB GridFS bucket
`analysis_files`). The evidence pack is assembled deterministically from the stored run
(`analysis_evidence.go`), the prompts live in `analysis_prompt.go`, and the request body used by
the article's experiments is recorded in every `experiments/results/exp-h/report-N/request.json`.

## Prerequisites

Install and verify these yourself before launching:

- **Go 1.26+**
- **Node.js 20.19+ or 22.12+** (with npm; required by Vite 7)
- **MongoDB 5.0+** running locally (the core creates time-series collections, which require 5.0+)
- **An OpenAI API key** (the article's analyses used the reasoning models o4-mini and o3)

To re-run the experiment scripts you additionally need Python 3.10+ with numpy, matplotlib and
requests (see the docstrings in `experiments/harness/`).

## Running IntelliSim locally

Keep the five folders as siblings — the Go modules reference the SDK by relative path
(`replace ... => ../intellisim.llm.provider.go.sdk`). All services read configuration from the
**process environment**; they do not load `.env` files themselves. Start each service in its own
terminal, in this order:

**1. MongoDB** — ensure it listens on `localhost:27017`.

**2. Auth mock** (port 9090):

```sh
cd intellisim.auth.mock
npm install
npm start
```

**3. Core backend** (HTTP 8080, gRPC 50051). `intellisim.core/.env-list` lists the commonly
tuned variables; everything else has sensible defaults in the module config structs. The three
below are the required minimum.

PowerShell:

```powershell
cd intellisim.core
$env:MONGO_HOST="localhost:27017"; $env:MONGO_DATABASE="intellisim"; $env:AUTH_SERVER="http://localhost:9090"
go run ./cmd/intelli-sim
```

bash:

```sh
cd intellisim.core
export MONGO_HOST=localhost:27017 MONGO_DATABASE=intellisim AUTH_SERVER=http://localhost:9090
go run ./cmd/intelli-sim
```

**4. OpenAI provider** (connects to the core's gRPC port). Variable reference:
`intellisim.openai.llm.provider/.env.example`.

PowerShell:

```powershell
cd intellisim.openai.llm.provider
$env:OPENAI_API_KEY="<your key>"
go run ./cmd/openai-provider
```

bash:

```sh
cd intellisim.openai.llm.provider
export OPENAI_API_KEY=<your key>
go run ./cmd/openai-provider
```

**5. Frontend** (port 3000):

```sh
cd intellisim.front
npm install
npm run dev
```

Open http://localhost:3000 — login is auto-approved as the `dev-user` persona
(see `intellisim.auth.mock/README.md` for the admin persona and scripted tokens). A finished
simulation gets an **Analysis** tab with a **New analysis** action; the generated report appears
there and can be downloaded as PDF.

## The experiment (EXP-H and EXP-H2)

Ten finished simulations from the authors' earlier benchmark campaign (room evacuation,
RiMEA-type evacuation, LLM-decision evacuation, wildfire, spatial SIR epidemic) were analysed by
the Level-3 pipeline. For each run, three phenomena were measured from the exported data and
written into a ground-truth sheet **before** any report was generated (blinding gate); the sheets
and the run list were committed first, the analyses came second. Twelve reports were generated
on 2026-07-24 — ten primary on `o4-mini-2025-04-16` (five in English, five in Ukrainian) and two
stretch re-runs on `o3-2025-04-16` — and scored against the sheets with a 0/1/2 rubric per
phenomenon plus counts of factual errors and hallucinated phenomena.

| Scoring | Points, 10 primary reports | Factual errors | Hallucinated phenomena |
|---|---|---|---|
| Pass I — pre-registered (2026-07-24) | **54 / 60 (90 %)** | 7 (0.7 per report) | 1 |
| Pass II — blind, sheet-only (2026-09-08) | 46 / 60 (77 %) | 11 (1.1) | 9 |
| Pass III — blind, mixed with EXP-H2 arms (2026-09-23) | 40 / 60 (67 %) | 8 (0.8) | 11 |

Both pre-registered acceptance criteria (≥ 70 % of phenomenon points, ≤ 1 factual error per
report) held together only in pass I; passes II and III each met one of them. Inter-pass
agreement on the 30 phenomenon items: Cohen's κ = 0.44 (I–II), 0.65 (II–III), 0.24 (I–III).
Pooled over the three scorings, post hoc: 140 / 180 = 77.8 % (bootstrap 95 % CI 0.67–0.87).

**EXP-H2** (pre-registered addendum, 2026-09-23) regenerated the ten primary analyses with the
same requests and unchanged engine (arm F2: 44 / 60 in pass III) and generated ten reports
**without any run data** — model description and run metadata only, outside the meta-system (arm
A: 14 / 60, 23 %). The original reports beat arm A in 10 of 10 runs (sign test p ≈ 0.001); the
no-data reports earned 0 of 20 points on data-dependent items. An objective re-check of the 25
statements disputed in pass II against the exported series found 6 true, 10 partly true, 8
refuted and 1 not checkable, and traced a large share of the disagreement to the temporal
resolution of the ranges the rubric left undefined.

Cost of record (OpenAI billing × list price): EXP-H $1.03 for 12 reports (≈ $0.04 per o4-mini
report after excluding one interrupted, billed attempt); EXP-H2 $0.646 for 20 reports. Wall-clock
per report ≈ 30–80 s (105 s and 141 s for the two reports in which a 60 s provider time-out forced
a retry); the article prints «approximately a minute».

All three scoring passes were LLM-assisted (pass I sheets: `scorer=Claude (author)`; passes II
and III: `scorer=pass-2-blind` / `pass-3-blind`, see `experiments/results/exp-h-rescoring/PROTOCOL.md`
and `experiments/results/exp-h2/REPORT.md` §2); none of them is a human-rater validation, which the
article states. The data check is rater-free. The numbers of record for every value the article prints, with
their file-level provenance, are in
[`experiments/results/_article/KEY-NUMBERS.md`](experiments/results/_article/KEY-NUMBERS.md).

### Where each promised item lives

| Item named in the article's data-availability statement | Path |
|---|---|
| Meta-system code incl. Level 3, version 0.1.2 | `intellisim.core/`, `intellisim.front/`, `intellisim.llm.provider.go.sdk/`, `intellisim.openai.llm.provider/`, `intellisim.auth.mock/` |
| Experiment protocol and its addendum | `experiments/protocols/36-analysis-eval-protocol.md`, `experiments/protocols/37-exph2-regeneration-ablation-protocol.md` |
| Ground-truth sheets (10, pre-registered) | `experiments/results/exp-h/ground-truth/sheet-1..10.md` |
| Reports with generation manifests (model deployment ids, token usage, latency) | `experiments/results/exp-h/report-1..12/` + `manifest.json`; `experiments/results/exp-h2/f2-1..10/`, `a-1..10/` + `manifest.json` |
| Three scoring passes | `experiments/results/exp-h/report-N/scoring.md` (I); `experiments/results/exp-h-rescoring/scoring-2/`, `rubric.csv`, `kappa.md` (II); `experiments/results/exp-h2/scoring-3/`, `rubric3.csv`, `OUTCOMES.json` (III) |
| Objective data check | `experiments/results/exp-h-rescoring/DATA-CHECK.md`, `data-check.csv` |
| Experiment code | `experiments/harness/` |
| Subset of source-run exports | `experiments/results/source-runs/` (31 files, `MANIFEST.md`) |
| Article figures and tables, number registry | `experiments/results/_article/` |

### Reproducibility notes

- **Model deployments.** The reports were generated on `o4-mini-2025-04-16` and `o3-2025-04-16`,
  which OpenAI has announced for retirement (API shutdown on 2026-10-23 and 2026-12-11
  respectively). After those dates the generation step cannot be repeated on the same
  deployments; each report folder together with its manifest records the deployment id, the
  request body, per-report token totals (per HTTP attempt for arm A in `attempts.json`) and the
  summed latency of the four calls, and the reports themselves are archived, so scoring, κ, the
  data check, the tables and the figures remain fully re-derivable.
- **Harness.** The scripts in `experiments/harness/` are shipped exactly as they were run.
  Scoring, agreement, data check, pooled statistics, tables and figures (`exph_rescore.py`,
  `kappa.py`, `exph_datacheck.py`, `exph2_score.py`, `exph2_blind.py`, `exph_pooled.py`,
  `exph_tables.py`, `l3_figures.py`) read only files in this repository and reproduce the shipped
  result files; `l3_figures.py` also writes a 1200-dpi cache into `experiments/article/draft/_figcache/`
  (created on demand, git-ignored). The generation scripts (`exph2_preflight.py`, `exph2_ablate.py`,
  `exph2_regen.py`, `exph2_run.py`) additionally expect (a) a running IntelliSim stack, (b) the
  HTTP client `intellisim_client.py` from the sibling campaign harness, published in
  `intellisim-experiment-artifacts/experiments/harness/`, placed at `intellisim.experiments/harness/`
  in this repository's root (next to `experiments/`; the scripts resolve it as
  `experiments/../intellisim.experiments/harness`), and (c) the provider's `.env` with
  `OPENAI_API_KEY` in the shipped folder `intellisim.openai.llm.provider/.env` (PowerShell
  format, `$env:NAME="value"`; the file is git-ignored). `exph_cost.py` and `exph2_cost.py` read
  the authors' OpenAI usage exports, which contain account identifiers and are **not**
  redistributed; their outputs are quoted in `KEY-NUMBERS.md` §5 and §10 and in
  `experiments/results/exp-h2/COST.md`. `exph2_prereg.py` (one-time pre-registration tooling)
  looks for the protocol at its original path and does not run after the move to `protocols/`.
  `citation_gate.py` reads the article's citation register, which is not part of this archive.
- **Paths inside the copied documents.** Protocols, reports and `KEY-NUMBERS.md` were written in
  the authors' private campaign repository and cite paths relative to its root. In this archive
  `results/…` is `experiments/results/…`, `experiments/NN-….md` is `experiments/protocols/NN-….md`,
  and `harness/…` is `experiments/harness/…`. The documents were not rewritten, so that the
  SHA-256 values listed in `experiments/results/exp-h/PROVENANCE.md` still verify for every file
  included; the exceptions are listed in `experiments/README.md` → «Provenance».

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.

The DejaVu fonts embedded in the core for PDF rendering remain under their own license — see
[`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md).

## Citation

If you use this software or data, please cite the article above — see [`CITATION.cff`](CITATION.cff).
