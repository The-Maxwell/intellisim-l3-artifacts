# EXP-H2 — regeneration and model-only ablation of the EXP-H Level-3 analyses (pre-registered addendum)

> **Status: pre-registration.** Written and committed on 2026-09-23 **before any EXP-H2 generation call**. This commit is the pre-registration evidence (the only commit the author authorised for EXP-H2, D24). Addendum to protocol 36 (`experiments/36-analysis-eval-protocol.md`, EXP-H, 2026-07-24). Decisions: `DECISIONS.md` D24 (scope), D25 (cost/duration corrections), D26 (metadata disclosure). Deviations after this commit go to `DECISIONS.md` with a date; nothing below is changed after generation starts.

## 1. Purpose

EXP-H scored 10 primary Level-3 reports (o4-mini) against pre-registered ground-truth sheets: 54/60 (pass 1) and 46/60 (pass 2, blind) phenomenon points. Two questions stayed open, and one new one arose on 2026-09-23:

1. **Floor.** How many rubric points can a report earn **without the run's data**, from domain priors plus the model's own metadata? Without this reference the 77–90 % has no scale.
2. **Metadata leak (D26).** The engine sends the model's name and description (part of M) and the simulation name (part of R) in every prompt. For at least report 3 the description states a ground-truth phenomenon (§3). How much of the score can the metadata alone explain?
3. **Nondeterminism.** How stable are the reports and their scores when the same analysis is regenerated?

EXP-H2 answers (1)–(2) with **arm A** (model-only ablation) and (3) with **arm F2** (full regeneration). F2 is also A's same-day control.

## 2. Arms

Both arms cover the 10 primary EXP-H runs (reports 1–10), use the original language of each report (5 en, 5 uk), the original context message verbatim, model alias `o4-mini` (snapshot recorded per call; `o4-mini-2025-04-16` expected), default reasoning effort, and one cost-only deviation from EXP-H: **max 16 000 completion tokens per call**. In EXP-H any call that long would already have hit the provider's 60 s HTTP timeout (a runaway call cost ≈ 0,44 USD, D25), so the cap does not change successful calls.

**F2 — full regeneration through the unchanged engine.** `POST /simulations/:id/analyses` with `results/exp-h/report-N/request.json` verbatim, except `name = "EXP-H2 F2 report N"` and `llm_target.generation = {"max_tokens": {"uint": 16000}}`. Engine `intellisim.core` dev `5072b5f` (analysis, LLM and data code identical to the EXP-H engine `6bb5bd9`; later commits touch only run-time movement, licence, README). Provider `intellisim.openai.llm.provider` dev `777aeca`. Both run unmodified; no application code is changed for EXP-H2 (author's constraint).

**A — model-only ablation via a standalone replica.** Because the engine cannot drop the run data without a code change, arm A is produced by `harness/exph2_ablate.py`, a replica of the engine's Level-3 prompt construction that calls the OpenAI Chat Completions API directly (key read at runtime from the provider's `.env`, never printed or stored). It reproduces:

- the section plan (dynamics, spatial for spatial models, emergence), the section system prompts, objectives, JSON contract, the JSON reminder, the synthesis system prompt and structure; every static fragment is asserted at run time to occur verbatim in `analysis_prompt.go` / `analysis_llm.go`;
- the evidence payload restricted to `{"model": ModelSummary, "run": RunSummary}` built exactly as `summarizeModel` (Go field names and order, Go JSON encoding), with **RunSummary.StepsCompleted removed**;
- the synthesis prompt **without** «(steps completed: N)» and **without** the HEADLINE STATISTICS block;
- the engine's call semantics: sequential sections, 2 attempts per call on provider error or contract-invalid JSON, `extractJSON` trimming, case-insensitive contract parsing; the provider's request body (`model`, system + user messages, `response_format: json_object`, `max_completion_tokens`), 60 s HTTP timeout, 1/2/4 s retry backoff on network errors, `Retry-After` on 429, retries on 5xx.

What arm A's model sees: the model summary (name, description, archetypes with property names and types, rule/interaction/event counts, environment size, torus flag, global property names, shape count), the run's name, configured step limit, seed and status, and the context message. It sees **no** population series, property series, statistics, spatial snapshots or step counts. The system prompt still says «You are given AGGREGATED time series and statistics»; it is kept byte-identical on purpose, so the only manipulated factor is the evidence.

Limitation stated in advance: arm A runs outside the metasystem, through a replica of its prompts, so A and F2 differ in the evidence **and** in the executing software. The replica's section prompts were built and hashed before generation (dry run, §8), and the pilot checks that the live bodies equal the dry-run bodies.

## 3. Metadata that reaches the model in every arm (disclosure, D26)

The banned-word list of protocol 36 §1.2 applied to the context hint only. The model summary and run name below were sent to the model in EXP-H and are sent again in F2 and A. Verbatim values are in Appendix A. Items that the metadata states or strongly primes:

| Report | Metadata | Ground-truth item it bears on |
|---|---|---|
| 3 | model name `exp-B12-bottleneck`; description «… qualitative pass = congestion localizes upstream (zone-1 occupancy >> zone-2)» | sheet 3 P1 (congestion in Room 1, not Room 2), P2 (bottleneck element) |
| 4 | description «… specific flow vs density curve vs RiMEA reference points. N varied 50/150/300 between batteries»; hint «за різної щільності натовпу» | sheet 4 P2 (flow rises with density), P3 (no saturation); primes a density–flow reading of one run |
| 7, 8 | run names `exp-F-p0.50-rep1`, `exp-F-p0.70-rep1` (tree density) | sheet 7 P1–P3, sheet 8 P1–P3 (regime inferable from p vs p_c ≈ 0,59) |
| 1, 2, 5, 6 | description «SFPE room … 0.4 m/cell … 100 persons @ 1.0 cell/step (1.333 m/s) … stop when everyone has left»; run names `exp-E-archetype_decision-rep301`, `exp-E-individual_decision-rep301` | unit inference (0,30 s/step); sheets 5–6 P3 (evacuation completes) |
| 9, 10 | description «spatial SIR … recovery confers immunity» | sheets 9–10 P3 (S non-increasing, R non-decreasing) |

## 4. A-priori item partition and predictions (fixed before generation)

Each of the 30 primary items is classified by what a report can reach **without run data**:

- **M — metadata/prior can reach 2** (ground truth is qualitative, an invariant, or stated in metadata): 1P2, 2P2, 3P1, 4P2, 5P3, 6P3, 7P3, 8P2, 8P3, 9P3, 10P3 (11 items).
- **P — prior can reach at most 1** (direction guessable, quantitative part needs data): 1P1, 2P1, 3P2, 5P1, 7P1, 7P2, 8P1, 9P1, 10P1 (9 items).
- **D — data-dependent, expected 0**: 1P3, 2P3, 3P3, 4P1, 4P3, 5P2, 6P1, 6P2, 9P2, 10P2 (10 items).

Ceiling for arm A implied by the partition: 11 × 2 + 9 × 1 = 31/60 if the model states every prior confidently; 0/60 if it abstains everywhere. **Prediction:** D(A) between 5 and 25 of 60 points (central 12–18), concentrated in M items; ≥ 90 % of D-item scores equal 0; A reports contain more hallucinated phenomena than the originals; D(F2) ≥ 0,7.

## 5. Scoring (pass 3)

- **Set:** 30 reports — the 10 original EXP-H reports, 10 F2, 10 A — scored in one session.
- **Blinding:** each report is converted to one neutral JSON (`report.sections[*].{key,title,narrative,key_findings,causal_links,emergent_phenomena}`, `executive_summary`, `conclusions`, `recommendations`, plus `language`); id, name, dates, selection, llm_target, token usage, warnings, file and arm are removed. Reports get shuffled IDs (random seed recorded in `results/exp-h2/scoring-3/SEED.txt` before scoring; mapping hidden from scorers). Blinding to arm is partial by nature: A reports may say that no data were provided.
- **Rubric:** `results/exp-h-rescoring/RUBRIC-pass2.md` verbatim (protocol 36 §2 + the three pass-2 clarifications), plus one tagging instruction: a credited quote that is hedged as an expectation rather than an observation is marked `[expectation]` (scores unchanged by the tag).
- **Inputs per report:** the sheet without its «Scoring» section, the neutral report JSON, the pass-2 output template. Nothing else (no pass-1/2 scores, KEY-NUMBERS, step series, web).
- **Scorers:** LLM-assisted, as passes 1 and 2 (disclosed in the article): 10 fresh agents × 3 reports; no agent gets two reports of the same run.
- **Status of pass-3 scores:** passes 1 and 2 remain the headline for EXP-H; pass-3 scores of the originals serve only as the same-session comparator for A and F2 and as a third agreement measurement. No score of any pass is changed.

## 6. Outcomes

**Primary:** paired difference D(original, pass 3) − D(A, pass 3) per run (10 pairs, all values published) and a one-sided sign test.

**Secondary (descriptive):** points of A by item class (M / P / D) and number of 2-point items in A; factual errors and hallucinated phenomena per arm; D(F2, pass 3) and criterion 1 (≥ 70 %) under pass 3 for originals and F2; item-level exact agreement original vs F2; whether report 4's density–flow misreading recurs in F2-4; κ between pass 2 and pass 3 on the 30 original items; wall-clock, calls, failed attempts, cost per arm.

**Decision rules (reporting, not re-running):** everything above is reported whatever the result. If D(A) ≥ 0,5, the article states that the rubric's 1-point level does not discriminate data-grounded from prior-driven reports and reports the D-item subscore alongside D. If D(F2) < 0,7, the article states that criterion 1 did not replicate on regeneration. **No report is regenerated after scoring**, and no report is regenerated because of its content.

## 7. Procedure and spend gates

1. Dry run (no API call): build and hash all F2 bodies and all A section bodies (§8); assert no run data in A.
2. Commit this addendum (pre-registration).
3. **Pilot:** A-1 then F2-1. Gate G2: both completed without warnings, ≤ 0,06 USD each (token usage × list price), F2 echoes the cap, A-1's live section bodies byte-equal to the dry-run bodies.
4. **Battery:** reports 2–10, odd n → A then F2, even n → F2 then A, sequentially. Gate G3 after every report: stop if cumulative cost > 1,20 USD or failed attempts > 2 (A: logged; F2: inferred when wall-clock − summed latency > 30 s). Hard ceiling 2,50 USD billed. Expected ≈ 0,8 USD.
5. The author exports the OpenAI usage CSV the next day; `harness/exph2_cost.py` reconciles billing with the recorded usage (difference expected = failed attempts; a live check of D25).
6. Scoring (§5), analysis → `results/exp-h2/REPORT.md`, KEY-NUMBERS §10.

Prices (list, per 1 M tokens, re-verified 2026-09-23 at https://developers.openai.com/api/docs/pricing): o4-mini 1,10 USD input / 4,40 USD output. Shutdown of `o4-mini-2025-04-16`: 2026-10-23 (https://developers.openai.com/api/docs/deprecations).

## 8. Frozen artefacts (sha256 at pre-registration)

The hash block below is generated by `harness/exph2_prereg.py` from the files as they stand at commit time.

| File | sha256 |
|---|---|
| `harness/exph2_common.py` | `39dccd06532e28e1c249f86e93092ff2bad0552d6c5f063d51ef5862341f1792` |
| `harness/exph2_ablate.py` | `c91bfbef3e35ac54c6166f84bcca4340e9e8c19b2be20d7f932dce571d148a7b` |
| `harness/exph2_regen.py` | `52ce068c901e51325956df65f50032bdc251ef989ccfaee55b59a72b3a6c8bdd` |
| `harness/exph2_run.py` | `d1c870e1581b20988d5667ef098ab5d581cba3c7e778d513c5ffaeb81ab5020b` |
| `harness/exph2_preflight.py` | `da4136ff1f68df99514155015df278953a8846ee5bd8df6211a297ed51ccd37e` |
| `harness/exph2_prereg.py` | `665f236a1e442f266a4a2537e4d8bccc046e9cb838518bb51ca9f472ae349910` |
| `results/exp-h-rescoring/RUBRIC-pass2.md` | `ff5213b227edbd1d3d930d973a3b6405b3fc668a156a601d2e5699a07f4b46a0` |
| `results/exp-h2/preflight.json` | `579e368b03afbd97df4f98b53a139481a1063d2e21a05c6e8f37c1197d062bd1` |
| `results/exp-h2/dryrun/a-hashes.json` | `e73f40efcf8a9d9835333806094815a0a0f3dbdc0fa8106218b292dd617d7aa6` |
| `results/exp-h2/dryrun/f2-hashes.json` | `14b17f0a5c1618c2e3b04975578d20ee3ece8da8a7194e973e9efbff124413a1` |
| `results/exp-h2/dryrun/a-1/req-dynamics.json` | `042ce449c089ab4af49ab1df2f95abc400bc9ce22a8fdeea1b3864fd89b610d5` |
| `results/exp-h2/dryrun/a-1/req-emergence.json` | `2639f317ecb68a76fdda3d52411572d0c58b25ace9434c91d6b188bdc8aea9b8` |
| `results/exp-h2/dryrun/a-1/req-spatial.json` | `b4ec7158a902adb5fee10ea75c0753267ace27ea95a94efbd61ef75fbc17bd67` |
| `results/exp-h2/dryrun/f2-1/request.json` | `f89737341dd5d6825169a7a19ce4754df266cd8fde7a3c80895991f5c803b257` |
| `results/exp-h2/dryrun/a-2/req-dynamics.json` | `56c96c991a9f87f5b91070b3c75419eabd86cbfdfcc04f0e4eed184a029c793d` |
| `results/exp-h2/dryrun/a-2/req-emergence.json` | `c414ac846724103a819d60282462bc82c0935a90974c9f7b76458fa8cd0f815f` |
| `results/exp-h2/dryrun/a-2/req-spatial.json` | `9008c72ea7b6d36f2b11aa7496f37f30edb79ca4cc25f01626d783a30cfe9db2` |
| `results/exp-h2/dryrun/f2-2/request.json` | `44f0a6f9c8dbc43e1d4caa28e257ac914e28354560e0a0be0bb35bb2ebfcdb1b` |
| `results/exp-h2/dryrun/a-3/req-dynamics.json` | `f524823095b5437969bd961081345dd3a2989ede2f825b4e768262b609ced593` |
| `results/exp-h2/dryrun/a-3/req-emergence.json` | `ea048126cfd6329e874af773edd0bf9dcee28438fd45741f108d18ed4a094263` |
| `results/exp-h2/dryrun/a-3/req-spatial.json` | `ca34741aa10513371c8866aaa2fbe190504ad19a6b69f6c49cd31a4ea5b49929` |
| `results/exp-h2/dryrun/f2-3/request.json` | `05a597b551e751363fe03a89fb2e0b19f5d4c5154552404d690c074348fe2020` |
| `results/exp-h2/dryrun/a-4/req-dynamics.json` | `f4f27a9740be8de52a9aedd9487323ed0022e960a09c62d4e1eaf348dfc79bb9` |
| `results/exp-h2/dryrun/a-4/req-emergence.json` | `0e4fb0bf9779901951639580961a2f60e406b1ef77cf8511ed7e81868ac2402a` |
| `results/exp-h2/dryrun/a-4/req-spatial.json` | `84709999c914e1292893a12852cdc48aa865b99e136f22442ee234725c8385e8` |
| `results/exp-h2/dryrun/f2-4/request.json` | `e0158caa32d4a0835de11b290c6810ba3546eed7b4aa6730f8333df2d56c913b` |
| `results/exp-h2/dryrun/a-5/req-dynamics.json` | `a4b39ffa3182e588030abfa2292c2b282e9ca685b265a18debd70e27a262ca01` |
| `results/exp-h2/dryrun/a-5/req-emergence.json` | `fe205a87116b646b9d9d82b61f7e206818ceb9a4bfdd470b8b4b9ddb847be815` |
| `results/exp-h2/dryrun/a-5/req-spatial.json` | `2058ddbf7f51bd4274f5e84823ca969f2febe5060892fabe7d58b2191d5e642e` |
| `results/exp-h2/dryrun/f2-5/request.json` | `76b8ca9d56b4a2fa65272bb8556be06b682ee2afa9c4464047e89157b62b75cf` |
| `results/exp-h2/dryrun/a-6/req-dynamics.json` | `81491edc78b172dbd5e0e70ec4f9dc4fb459a70011d92d61508f32fe66722c4d` |
| `results/exp-h2/dryrun/a-6/req-emergence.json` | `bc62d1de2a4c2919bf2e856f4d18591013e291b4d0f9dcab20fe65498722eb98` |
| `results/exp-h2/dryrun/a-6/req-spatial.json` | `8d84a421453896676eeb9bcd212e025c61cf360718107af353564b29ed24c9f6` |
| `results/exp-h2/dryrun/f2-6/request.json` | `ed0402fd93375053e758e4b6cbf2afe37886e8c309269b33fadac8adae1ed250` |
| `results/exp-h2/dryrun/a-7/req-dynamics.json` | `bd1e6c38834b53c879505fd6507619d32b78e635a35168523dc1b1449a1542d4` |
| `results/exp-h2/dryrun/a-7/req-emergence.json` | `0db82a06b990b8900f8f6092615d5fe36a9cb32a46fd3f38e683e81254444557` |
| `results/exp-h2/dryrun/a-7/req-spatial.json` | `4c07f2feeeadfcebebaa2d5bd1eff99715741a4a446883f5d68dc4f09b1cc86f` |
| `results/exp-h2/dryrun/f2-7/request.json` | `34eec315b889cf84bfd15c65abc66c2a000f196de4e5c7aa7b886b4e7f3254b8` |
| `results/exp-h2/dryrun/a-8/req-dynamics.json` | `0d6a3bfd438e69638b3af776cfe44bd26fb12024f9425f611097c59635535d9d` |
| `results/exp-h2/dryrun/a-8/req-emergence.json` | `bac99f7706f60b63cfd41c03163961030f0016faec6362ef06e853086e2898e0` |
| `results/exp-h2/dryrun/a-8/req-spatial.json` | `78a09349d2d33658ed77179ebd22d1b7bcff1ba3356da2728d30f59456514a0d` |
| `results/exp-h2/dryrun/f2-8/request.json` | `1e973afccc1d22d9fe0f024c2b9e73fa1390c7797e7a1fb52574702a0f89e356` |
| `results/exp-h2/dryrun/a-9/req-dynamics.json` | `3854ee6d56e6e3cefcca7bba35e5a0ced3c7a6d672c54aac643c38986a8e3fa1` |
| `results/exp-h2/dryrun/a-9/req-emergence.json` | `e17dec017bcd8035d2641c7f7c2c38d05c6a9ad0ef25def6fb640b609a04e0ae` |
| `results/exp-h2/dryrun/a-9/req-spatial.json` | `c3bd701e88f0d110319deffd5e8cb3ab421082cdda43f1ff51268399df0d8604` |
| `results/exp-h2/dryrun/f2-9/request.json` | `99697deddd3f005a59892f0070f4b4173b02ed3132e24f357a1fdb10c1a07798` |
| `results/exp-h2/dryrun/a-10/req-dynamics.json` | `0942ae677019185364d6f0000aa25d7cdb9a265a65d731d14ed5ba05a5e4b79d` |
| `results/exp-h2/dryrun/a-10/req-emergence.json` | `45ec7149692c3dfa3be5ba8b010f360110a121ed39d4a706cfde1ed6546b3659` |
| `results/exp-h2/dryrun/a-10/req-spatial.json` | `bc243ddb74aeb5e18437c3cee97b1a85ab8cd8a79bd897525c9e697e56e0d7e6` |
| `results/exp-h2/dryrun/f2-10/request.json` | `d080abb00dc3662daafb7a5800fbc243f49c4e7a84632d4ebbb2ff4ec153d5b0` |

## Appendix A. Metadata sent to the model (verbatim, from the dry-run evidence payload)

The same model summary and run name reach the model in EXP-H, F2 and A (A additionally lacks the run data). Context messages are those of protocol 36 §1.2.

**Report 1** (en) — run `exp-C-rep3` (ConfiguredSteps 900, seed 103, status finished); model `exp-evac-V2-reference`.

> Description: Frozen reference evacuation model V2 (SFPE room, 30-model-spec.md): 40x40 grid @ 0.4 m/cell, interior 38x38 = 15.2x15.2 m, 3-cell (1.2 m) east door at x=39 y=18..20, 100 persons @ 1.0 cell/step (1.333 m/s), block collision, stop when everyone has left. As-built from EXP-A attempt 7 + manual edits (D27).

> Context message: Analyze this simulation's results. Context: People are evacuating a single room through one door. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.

**Report 2** (uk) — run `exp-C-rep18` (ConfiguredSteps 900, seed 118, status finished); model `exp-evac-V2-reference`.

> Description: Frozen reference evacuation model V2 (SFPE room, 30-model-spec.md): 40x40 grid @ 0.4 m/cell, interior 38x38 = 15.2x15.2 m, 3-cell (1.2 m) east door at x=39 y=18..20, 100 persons @ 1.0 cell/step (1.333 m/s), block collision, stop when everyone has left. As-built from EXP-A attempt 7 + manual edits (D27).

> Context message: Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з одного приміщення через одні двері. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.

**Report 3** (en) — run `exp-B12-rep1` (ConfiguredSteps 600, seed 1201, status finished); model `exp-B12-bottleneck`.

> Description: EXP-B stretch Test 12 / V7 (30-model-spec.md §8): Room 1 (150 persons, 10x10 m) -> 1.2 m door -> 2 m corridor -> Room 2 -> 1.2 m exit; qualitative pass = congestion localizes upstream (zone-1 occupancy >> zone-2). Derived from exp-evac-V2-reference (D27/D30).

> Context message: Analyze this simulation's results. Context: 150 people move out of a building section consisting of two rooms connected by a corridor. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.

**Report 4** (uk) — run `exp-B4-N300-rep1` (ConfiguredSteps 900, seed 421, status finished); model `exp-B4-flow`.

> Description: EXP-B stretch Test 4 / V9 (30-model-spec.md §8): 16x12 m holding room feeding a 4.0 m (10-cell) opening; assigned speed 1.10 m/s = 0.825 cells/step; density measured in a 4x2 m rectangle upstream; specific flow vs density curve vs RiMEA reference points. N varied 50/150/300 between batteries.

> Context message: Проаналізуйте результати цієї симуляції. Контекст: Люди проходять крізь широкий отвір за різної щільності натовпу. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.

**Report 5** (en) — run `exp-E-archetype_decision-rep301` (ConfiguredSteps 300, seed 301, status finished); model `exp-E-decision`.

> Description: Frozen reference evacuation model V2 (SFPE room, 30-model-spec.md): 40x40 grid @ 0.4 m/cell, interior 38x38 = 15.2x15.2 m, 3-cell (1.2 m) east door at x=39 y=18..20, 100 persons @ 1.0 cell/step (1.333 m/s), block collision, stop when everyone has left. As-built from EXP-A attempt 7 + manual edits (D27).

> Context message: Analyze this simulation's results. Context: People are evacuating a room through one door; some agent decisions during the run are made by a language model. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.

**Report 6** (uk) — run `exp-E-individual_decision-rep301` (ConfiguredSteps 300, seed 301, status finished); model `exp-E-individual`.

> Description: Frozen reference evacuation model V2 (SFPE room, 30-model-spec.md): 40x40 grid @ 0.4 m/cell, interior 38x38 = 15.2x15.2 m, 3-cell (1.2 m) east door at x=39 y=18..20, 100 persons @ 1.0 cell/step (1.333 m/s), block collision, stop when everyone has left. As-built from EXP-A attempt 7 + manual edits (D27).

> Context message: Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з приміщення через одні двері; частину рішень агентів під час виконання ухвалює мовна модель. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.

**Report 7** (uk) — run `exp-F-p0.50-rep1` (ConfiguredSteps 400, seed 800, status finished); model `exp-F-fire-reference-61`.

> Description: An agent-based forest fire model on a 101x101 grid. Trees are placed at a specified density. Fire starts at the left edge and spreads orthogonally. Burned-out trees cannot catch fire again. Simulation stops when no trees are burning.

> Context message: Проаналізуйте результати цієї симуляції. Контекст: Вогонь поширюється лісом. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.

**Report 8** (en) — run `exp-F-p0.70-rep1` (ConfiguredSteps 400, seed 800, status finished); model `exp-F-fire-reference-61`.

> Description: An agent-based forest fire model on a 101x101 grid. Trees are placed at a specified density. Fire starts at the left edge and spreads orthogonally. Burned-out trees cannot catch fire again. Simulation stops when no trees are burning.

> Context message: Analyze this simulation's results. Context: Fire spreads through a forest of trees. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.

**Report 9** (en) — run `exp-G-med-rep1` (ConfiguredSteps 2000, seed 8501, status finished); model `exp-G-med-b0.18-model`.

> Description: A spatial SIR (Susceptible-Infected-Recovered) disease spread model where 200 people move randomly in a 50x50 toroidal grid. Infection spreads through proximity, recovery confers immunity, and simulation ends when all infected have recovered.

> Context message: Analyze this simulation's results. Context: An infectious disease spreads in a population of moving agents. Describe the observed dynamics, quantify the key outcomes, and note anything notable or unexpected.

**Report 10** (uk) — run `exp-G-med-rep8` (ConfiguredSteps 2000, seed 8508, status finished); model `exp-G-med-b0.18-model`.

> Description: A spatial SIR (Susceptible-Infected-Recovered) disease spread model where 200 people move randomly in a 50x50 toroidal grid. Infection spreads through proximity, recovery confers immunity, and simulation ends when all infected have recovered.

> Context message: Проаналізуйте результати цієї симуляції. Контекст: Інфекційне захворювання поширюється в популяції рухомих агентів. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.

