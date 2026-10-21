"""EXP-H2 pass 3 — build the blinded scoring set (protocol 37 §5).

30 reports (10 original EXP-H + 10 F2 + 10 A) -> one neutral JSON each
    {"language", "report": {sections[*].{key,title,narrative,key_findings,causal_links,emergent_phenomena},
                            executive_summary, conclusions, recommendations}}
(no id / name / dates / selection / llm_target / token usage / warnings / file / arm).

Randomisation: a seed from `secrets` is written to results/exp-h2/scoring-3/SEED.txt BEFORE any scoring;
it drives (i) the random report IDs R01..R30 and (ii) the scorer design: 10 agents x 3 reports,
report (run r, arm k) -> agent (pi(r) + off_k) mod 10 with distinct offsets, so every agent gets three
different runs, one report of each arm, in shuffled order. The mapping is written to
results/exp-h2/scoring-3/_mapping.json and is NOT placed in any scorer folder.

Scorer folders (outside the repo): <scratch>/agent-GG/{RUBRIC.md, INSTRUCTIONS.md, Rxx/{sheet.md, report.json, TEMPLATE.md}}
Usage: python harness/exph2_blind.py --scratch <dir>
"""

from __future__ import annotations

import argparse
import json
import random
import secrets
import sys
from datetime import date
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from exph2_common import EXPH, OUT, PRIMARY, REPO, console_utf8, load_primary, utc_now, write_json  # noqa: E402

S3 = OUT / "scoring-3"
ARMS = ("orig", "F2", "A")
RUBRIC_SRC = REPO / "results" / "exp-h-rescoring" / "RUBRIC-pass2.md"
TAG_NOTE = ("\n\nPass-3 addition (tagging only, scores unchanged): when a quote you credit is phrased as an "
            "expectation or hypothesis rather than as an observation of this run (e.g. \"would\", \"is expected to\", "
            "\"likely\", \"typically\"), append the tag `[expectation]` after that quote.\n")


def load_report(n: int, arm: str) -> dict:
    if arm == "orig":
        _, a = load_primary(n)
    else:
        a = json.loads((OUT / f"{arm.lower()}-{n}" / "report.json").read_text(encoding="utf-8"))
    r = a["report"]
    secs = [{k: s.get(k) for k in ("key", "title", "narrative", "key_findings", "causal_links", "emergent_phenomena")}
            for s in r.get("sections") or []]
    return {"language": a["language"],
            "report": {"sections": secs, "executive_summary": r.get("executive_summary", ""),
                       "conclusions": r.get("conclusions") or [], "recommendations": r.get("recommendations") or []}}


def sheet_without_scoring(n: int) -> str:
    t = (EXPH / "ground-truth" / f"sheet-{n}.md").read_text(encoding="utf-8")
    cut = t.find("\n---\n## Scoring")
    if cut < 0:
        raise SystemExit(f"sheet-{n}: scoring section marker not found")
    return t[:cut].rstrip() + "\n"


def template(rid: str, lang: str) -> str:
    return (f"# EXP-H scoring - report {rid}\n\n"
            f"Meta: sim_id={rid} analysis_id={rid} lang={lang} llm_target=o4-mini generated=na scorer=pass-3-blind scored={date.today().isoformat()}\n\n"
            "| # | Score 0/1/2 | Evidence (verbatim report quote) |\n|---|---|---|\n| 1 | | |\n| 2 | | |\n| 3 | | |\n\n"
            "**Phenomenon points: X/6**\n\n"
            "Factual errors (verbatim + what the data says): count = X\n- ...\n\n"
            "Hallucinated phenomena (verbatim + why unsupported): count = Y\n- ...\n\n"
            "Actionable recommendations: y/n - quote: \"...\"\n\n"
            "Language/format notes: ...\n")


INSTRUCTIONS = """# Scoring instructions (pass 3)

You score three analysis reports, each against its own pre-registered ground-truth sheet.

Read ONLY the files in this folder: `RUBRIC.md` and, for each report ID, `<ID>/sheet.md`, `<ID>/report.json`,
`<ID>/TEMPLATE.md`. Do not open any other file or directory, and do not use the web.

For each report ID:
1. Read the whole `report.json` (everything under the `report` key) before scoring.
2. Score the three phenomena of `sheet.md` with the rubric (2 / 1 / 0), citing verbatim quotes.
3. Count factual errors and hallucinated phenomena, each with a verbatim quote and what the sheet's data say.
4. Record whether actionable recommendations are present (y/n + one quote).
5. Write your result as `<ID>.md` in this folder, following `TEMPLATE.md` exactly (keep the Meta line unchanged,
   fill the table, the `**Phenomenon points: N/6**` line and both `count = N` lines).

Be conservative and consistent. Score against the sheet's measured values and data-supported ranges,
not against textbook expectations. The three reports are independent; score each on its own.
"""


def main() -> int:
    console_utf8()
    ap = argparse.ArgumentParser()
    ap.add_argument("--scratch", required=True)
    a = ap.parse_args()
    scratch = Path(a.scratch)
    if (S3 / "SEED.txt").exists():
        raise SystemExit("scoring-3/SEED.txt exists — the blinded set is built once (protocol 37 §5)")
    seed = secrets.randbits(32)
    S3.mkdir(parents=True, exist_ok=True)
    (S3 / "SEED.txt").write_text(f"{seed}\nwritten {utc_now()} before any pass-3 scoring\n", encoding="utf-8")
    rng = random.Random(seed)
    items = [(n, arm) for n in PRIMARY for arm in ARMS]
    ids = [f"R{i:02d}" for i in range(1, 31)]
    rng.shuffle(ids)
    rid = dict(zip(items, ids))
    perm = PRIMARY[:]
    rng.shuffle(perm)
    offsets = rng.sample(range(10), 3)
    groups: dict[int, list[tuple[int, str]]] = {g: [] for g in range(10)}
    for n in PRIMARY:
        for k, arm in enumerate(ARMS):
            groups[(perm.index(n) + offsets[k]) % 10].append((n, arm))
    rubric = RUBRIC_SRC.read_text(encoding="utf-8") + TAG_NOTE
    mapping = {"seed": seed, "offsets": offsets, "run_permutation": perm, "reports": [], "agents": {}}
    for g, members in groups.items():
        assert len(members) == 3 and len({n for n, _ in members}) == 3 and {arm for _, arm in members} == set(ARMS)
        rng.shuffle(members)
        d = scratch / f"agent-{g + 1:02d}"
        d.mkdir(parents=True, exist_ok=True)
        (d / "RUBRIC.md").write_text(rubric, encoding="utf-8")
        (d / "INSTRUCTIONS.md").write_text(INSTRUCTIONS, encoding="utf-8")
        mapping["agents"][f"agent-{g + 1:02d}"] = [rid[m] for m in members]
        for n, arm in members:
            rep = load_report(n, arm)
            rd = d / rid[(n, arm)]
            rd.mkdir(exist_ok=True)
            (rd / "sheet.md").write_text(sheet_without_scoring(n), encoding="utf-8")
            (rd / "report.json").write_text(json.dumps(rep, ensure_ascii=False, indent=2), encoding="utf-8")
            (rd / "TEMPLATE.md").write_text(template(rid[(n, arm)], rep["language"]), encoding="utf-8")
            mapping["reports"].append({"id": rid[(n, arm)], "report_no": n, "arm": arm, "agent": f"agent-{g + 1:02d}"})
    write_json(S3 / "_mapping.json", mapping)
    print(f"seed={seed}; 30 reports in 10 agent folders under {scratch}")
    for g, v in mapping["agents"].items():
        print(g, v)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
