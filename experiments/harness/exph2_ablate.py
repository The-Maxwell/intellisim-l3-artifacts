"""EXP-H2 arm A — model-only ablation via a standalone replica of the engine's Level-3 prompts.

The author forbade any change to core/provider/app code (D24), so the engine cannot be told to
drop the run data. This script rebuilds, outside the engine, exactly what the engine sends for a
default EXP-H analysis — the same system prompts, section plan, JSON contract, reminder and
synthesis — with ONE difference: the evidence payload keeps only
    model = ModelSummary (name, description, archetypes, counts, environment)   [analysis_evidence.go:40-71, 272-299]
    run   = RunSummary WITHOUT StepsCompleted                                    [analysis_evidence.go:73-79, 94]
and the synthesis omits "(steps completed: N)" and the HEADLINE STATISTICS block
[analysis_prompt.go:103, 118-119]. No population / property series / statistics / spatial
snapshots reach the model.

Replicated engine behaviour (intellisim.core @ 5072b5f, analysis code identical to EXP-H's 6bb5bd9):
  * sections run sequentially in the planned order (dynamics, spatial if IsSpatial, emergence),
    then synthesis; section title = LLM title or the guide title          [analysis_pipeline.go:64-112]
  * callJSON: 2 attempts (SIMULATIONS_ANALYSIS_MAX_RETRIES=1); an attempt fails on a provider
    error or on JSON that does not unmarshal into the contract            [analysis_llm.go:30-66]
  * extractJSON trimming                                                   [analysis_llm.go:112-124]
  * provider request = Chat Completions {model, messages[system,user], response_format json_object,
    max_completion_tokens}; network errors/timeouts retried after 1/2/4 s, 429 honours Retry-After,
    5xx retried, other 4xx not retried; HTTP timeout 60 s                  [provider openai.go:167-235, 343-500]
Every static prompt fragment is asserted, at run time, to occur verbatim in the Go source.

Usage:
  python harness/exph2_ablate.py --dry-run [--reports 1-10]   # build + archive section prompts, no API call
  python harness/exph2_ablate.py --run --report N             # one live report (called by exph2_run.py)
The OpenAI key is read from the provider's .env at call time and is never printed, logged or stored.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import time
from pathlib import Path

import requests

sys.path.insert(0, str(Path(__file__).resolve().parent))
from exph2_common import (EXPH, MAX_TOKENS, MODEL_UID, OUT, SIBLING_HARNESS, WORKSPACE,  # noqa: E402
                          console_utf8, cost_usd, go_json, load_primary, provider_setting,
                          utc_now, write_json)

sys.path.insert(0, str(SIBLING_HARNESS))
from intellisim_client import IntelliSimClient  # noqa: E402

CORE = WORKSPACE / "intellisim.core" / "modules" / "simulations"
GO_SOURCES = [CORE / "analysis_prompt.go", CORE / "analysis_llm.go"]

# ---------------------------------------------------------------- engine text (verified below)

SYS_HEAD = "You are an expert analyst of agent-based modeling (ABM) simulations working inside the IntelliSim meta-system.\n\n"
SYS_ONTOLOGY = [
    "DOMAIN ONTOLOGY: A model defines agent ARCHETYPES (templates) with typed PROPERTIES (number/string/enum/boolean/color) and a state machine; ",
    "a RULE ENGINE (condition types incl. property/position/proximity/state/count/decision; action types incl. set/modify property, move, transition, spawn/remove, emit_event, call_llm); ",
    "INTERACTIONS (agent-agent/shape/global); EVENTS; and a spatial ENVIRONMENT (grid, optional torus, shapes, global properties). ",
    "A simulation runs in discrete steps; per-step state was recorded to a time series.\n\n",
]
SYS_TASK_FMT = "YOUR TASK — section \"%s\": %s\n\n"
SYS_DATA = ("DATA: You are given AGGREGATED time series and statistics (not raw per-agent frames). Ground EVERY claim strictly "
            "in the provided evidence; never invent numbers or entities. Cite concrete numbers and step indices/ranges from the "
            "evidence when making claims. Prefer causal and emergent insight over restating values. Be concise, specific, and structured.\n\n")
SYS_OUTPUT = "OUTPUT: Respond with a SINGLE JSON object and nothing else, in this exact shape:\n"
SYS_SHAPE = '{"title": string, "narrative": string, "key_findings": [string], "causal_links": [string], "emergent_phenomena": [string]}'
SYS_BULLETS = ("- narrative: 2–5 paragraphs of analysis.\n- key_findings: 3–7 short bullet insights.\n"
               "- causal_links: cause→effect statements (may be empty if none are evidenced).\n"
               "- emergent_phenomena: system-level behaviors not reducible to individual rules; may be empty.\n")
SYS_LANG_FMT = "Write all natural-language content in %s.\n"

USER_EVIDENCE_OPEN = "=== SIMULATION EVIDENCE (JSON) ===\n"
USER_EVIDENCE_CLOSE = "\n=== END EVIDENCE ===\n"
USER_CONTEXT_FMT = "\nRESEARCHER CONTEXT / QUESTIONS:\n%s\n"
USER_PRODUCE_FMT = "\nProduce the \"%s\" section now as the specified JSON."
JSON_REMINDER = "\n\nReturn ONLY the JSON object, with no markdown fences or commentary."

SYN_SYS = [
    "You are an expert ABM simulation analyst writing the conclusion of a results-analysis report for a researcher.\n\n",
    "You are given the section analyses already produced for this simulation plus its headline statistics. ",
    "Synthesize them into an executive summary, overall conclusions, and actionable recommendations. Ground everything in the provided material; do not introduce unsupported claims.\n\n",
    "OUTPUT: Respond with a SINGLE JSON object and nothing else, in this exact shape:\n",
]
SYN_SHAPE = '{"executive_summary": string, "conclusions": [string], "recommendations": [string]}'
SYN_BULLETS = "- executive_summary: 1–2 tight paragraphs.\n- conclusions: 3–6 bullets.\n- recommendations: 3–6 actionable bullets (model/parameter/further-investigation suggestions).\n"
SYN_LANG_FMT = "Write all content in %s.\n"
# engine: "SIMULATION: %s (steps completed: %d)\n\n" -> model-only drops the parenthesis (the one deliberate change)
SYN_SIM_FMT_ENGINE = "SIMULATION: %s (steps completed: %d)\n\n"
SYN_SECTIONS_OPEN = "=== SECTION ANALYSES ===\n"
SYN_SEC_FMT = "## %s\n%s\n"
SYN_KF_FMT, SYN_CL_FMT, SYN_EP_FMT = "Key findings: %s\n", "Causal links: %s\n", "Emergent phenomena: %s\n"
SYN_STATS_OPEN = "=== HEADLINE STATISTICS (JSON) ===\n"          # verified to exist; NOT sent in arm A
SYN_PRODUCE = "\nProduce the synthesis now as the specified JSON."

GUIDE = {
    "dynamics": ("Population & Property Dynamics",
                 "Analyze how agent populations and key numeric properties evolved over time. "
                 "Identify growth/decline phases, oscillations, equilibria, tipping points, and correlations between series."
                 " If shape_property_series is present in the evidence, treat environment-shape properties (resources, water bodies, zones) as first-class dynamics and relate their trends to agent behavior."),
    "spatial": ("Spatial & Interaction Patterns",
                "Analyze spatial distribution, clustering, movement and agent interactions. "
                "Identify aggregation/segregation, territory formation, and how spatial structure shaped outcomes."),
    "emergence": ("Emergent Behavior & Causal Relationships",
                  "Identify EMERGENT phenomena (system-level behavior not reducible to individual rules) and "
                  "establish CAUSE-AND-EFFECT chains between events. Go beyond restating statistics: explain WHY the system behaved as it did. "
                  "Place each distinct emergent phenomenon — self-organisation, phase transitions, oscillations, cascades, etc. — as a concise statement in the emergent_phenomena list."
                  " If shape_property_series is present, consider agent–environment feedback loops (e.g. agents depleting or replenishing shape resources) as candidate causal chains."),
}


def _go_literal_stream() -> str:
    """Concatenation of every Go string literal (interpreted + raw) in the analysis sources, decoded."""
    out = []
    lit = re.compile(r'"((?:[^"\\\n]|\\.)*)"|`([^`]*)`')
    esc = {"n": "\n", "t": "\t", "r": "\r", '"': '"', "\\": "\\", "'": "'"}
    for src in GO_SOURCES:
        for m in lit.finditer(src.read_text(encoding="utf-8")):
            if m.group(2) is not None:
                out.append(m.group(2))
            else:
                out.append(re.sub(r"\\(.)", lambda e: esc.get(e.group(1), "\\" + e.group(1)), m.group(1)))
    return "".join(out)


def verify_against_go_source() -> int:
    stream = _go_literal_stream()
    frags = [SYS_HEAD, *SYS_ONTOLOGY, SYS_TASK_FMT, SYS_DATA, SYS_OUTPUT, SYS_SHAPE, SYS_BULLETS, SYS_LANG_FMT,
             USER_EVIDENCE_OPEN, USER_EVIDENCE_CLOSE, USER_CONTEXT_FMT, USER_PRODUCE_FMT, JSON_REMINDER,
             *SYN_SYS, SYN_SHAPE, SYN_BULLETS, SYN_LANG_FMT, SYN_SIM_FMT_ENGINE, SYN_SECTIONS_OPEN,
             SYN_SEC_FMT, SYN_KF_FMT, SYN_CL_FMT, SYN_EP_FMT, SYN_STATS_OPEN, SYN_PRODUCE,
             "English", "Ukrainian"]
    for t, o in GUIDE.values():
        frags += [t, o]
    missing = [f for f in frags if f not in stream]
    if missing:
        raise SystemExit("prompt fragments not found verbatim in Go source: " + repr(missing[:3]))
    return len(frags)


# ---------------------------------------------------------------- evidence (model-only)

def _nil_if_empty(xs):
    return xs if xs else None     # Go nil slice -> null (bson omitempty drops empty arrays)


def model_summary(snap: dict) -> dict:
    """ModelSummary exactly as summarizeModel() builds it, keys in Go struct-field order."""
    m = snap.get("model", snap)
    archetypes = []
    for a in m.get("agents") or []:
        props = [{"UID": p.get("uid", ""), "Name": p.get("name", ""), "Type": p.get("type", "")}
                 for p in (a.get("properties") or [])]
        archetypes.append({"UID": a.get("uid", ""), "Name": a.get("name", ""), "Type": a.get("type", ""),
                           "States": _nil_if_empty(a.get("states")), "Properties": _nil_if_empty(props)})
    env = m.get("environment")
    env_sum = None
    if env:
        gp = [{"UID": p.get("uid", ""), "Name": p.get("name", ""), "Type": p.get("type", "")}
              for p in (env.get("global_properties") or [])]
        env_sum = {"Width": env.get("width", 0), "Height": env.get("height", 0), "Torus": bool(env.get("torus")),
                   "GlobalProps": _nil_if_empty(gp), "ShapeCount": len(env.get("shapes") or [])}
    return {
        "Name": m.get("name", ""), "Description": m.get("description", ""), "IsSpatial": bool(m.get("is_spatial")),
        "UpdateMode": (m.get("simulation_config") or {}).get("update_mode", ""),
        "Archetypes": _nil_if_empty(archetypes), "RuleCount": len(m.get("rules") or []),
        "Interactions": len(m.get("interactions") or []), "Events": len(m.get("events") or []),
        "Environment": env_sum,
    }


def run_summary_model_only(sim: dict) -> dict:
    """RunSummary field order without StepsCompleted (the deliberate ablation)."""
    return {"Name": sim["name"], "ConfiguredSteps": sim["steps"], "Seed": sim["seed"], "Status": sim["status"]}


def build_context(n: int, c: IntelliSimClient) -> dict:
    req, orig = load_primary(n)
    sim_id = orig["simulation_id"]
    sim = c.get_simulation(sim_id)
    snap = c.get_model_snapshot(sim_id)
    model = model_summary(snap)
    plan = [s["key"] for s in orig["report"]["sections"]]
    expect = ["dynamics"] + (["spatial"] if model["IsSpatial"] else []) + ["emergence"]
    if plan != expect:
        raise SystemExit(f"report {n}: section plan {plan} != engine plan {expect}")
    return {"n": n, "sim_id": sim_id, "language": req["language"], "context": req["context_message"],
            "model": model, "run": run_summary_model_only(sim), "plan": plan}


def section_prompts(ctx: dict, key: str) -> tuple[str, str]:
    title, objective = GUIDE[key]
    lname = "Ukrainian" if ctx["language"] == "uk" else "English"
    system = (SYS_HEAD + "".join(SYS_ONTOLOGY) + SYS_TASK_FMT % (title, objective) + SYS_DATA + SYS_OUTPUT
              + SYS_SHAPE + "\n" + SYS_BULLETS + SYS_LANG_FMT % lname)
    user = USER_EVIDENCE_OPEN + go_json({"model": ctx["model"], "run": ctx["run"]}) + USER_EVIDENCE_CLOSE
    if ctx["context"].strip():
        user += USER_CONTEXT_FMT % ctx["context"].strip()
    user += USER_PRODUCE_FMT % title
    return system, user + JSON_REMINDER


def synthesis_prompts(ctx: dict, sections: list[dict]) -> tuple[str, str]:
    lname = "Ukrainian" if ctx["language"] == "uk" else "English"
    system = "".join(SYN_SYS) + SYN_SHAPE + "\n" + SYN_BULLETS + SYN_LANG_FMT % lname
    user = "SIMULATION: %s\n\n" % ctx["run"]["Name"] + SYN_SECTIONS_OPEN
    for s in sections:
        user += SYN_SEC_FMT % (s["title"], s["narrative"])
        if s.get("key_findings"):
            user += SYN_KF_FMT % "; ".join(s["key_findings"])
        if s.get("causal_links"):
            user += SYN_CL_FMT % "; ".join(s["causal_links"])
        if s.get("emergent_phenomena"):
            user += SYN_EP_FMT % "; ".join(s["emergent_phenomena"])
        user += "\n"
    if ctx["context"].strip():
        user += USER_CONTEXT_FMT % ctx["context"].strip()
    user += SYN_PRODUCE
    return system, user + JSON_REMINDER


def request_body(system: str, user: str) -> dict:
    return {"model": MODEL_UID, "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}],
            "max_completion_tokens": MAX_TOKENS, "response_format": {"type": "json_object"}}


# ---------------------------------------------------------------- JSON contract (Go-like unmarshal)

SECTION_FIELDS = {"title": str, "narrative": str, "key_findings": list, "causal_links": list, "emergent_phenomena": list}
SYNTH_FIELDS = {"executive_summary": str, "conclusions": list, "recommendations": list}


def extract_json(s: str) -> str:
    s = s.strip()
    for p in ("```json", "```"):
        if s.startswith(p):
            s = s[len(p):]
            break
    if s.endswith("```"):
        s = s[:-3]
    s = s.strip()
    a, b = s.find("{"), s.rfind("}")
    return s[a:b + 1] if a >= 0 and b > a else s


def go_unmarshal(text: str, fields: dict) -> dict | None:
    """Mimic json.Unmarshal into the contract struct: object required, keys matched
    case-insensitively, unknown keys ignored, wrong types -> error, null -> zero value."""
    try:
        obj = json.loads(extract_json(text))
    except json.JSONDecodeError:
        return None
    if not isinstance(obj, dict):
        return None
    out = {k: ([] if t is list else "") for k, t in fields.items()}
    lower = {k.lower(): v for k, v in obj.items()}
    for k, t in fields.items():
        if k not in lower or lower[k] is None:
            continue
        v = lower[k]
        if t is str and not isinstance(v, str):
            return None
        if t is list and (not isinstance(v, list) or any(x is not None and not isinstance(x, str) for x in v)):
            return None
        out[k] = [x or "" for x in v] if t is list else v
    return out


# ---------------------------------------------------------------- live calls

class Caller:
    def __init__(self, log_dir: Path):
        self.url = provider_setting("OPENAI_BASE_URL").rstrip("/") + "/chat/completions"
        self.timeout = float(provider_setting("OPENAI_TIMEOUT_SECONDS"))
        self.retries = int(provider_setting("OPENAI_MAX_RETRIES"))
        self.log_dir = log_dir
        self.attempts: list[dict] = []

    def _http(self, body: dict, tag: str) -> tuple[str | None, dict | None]:
        """One provider-level invocation (doRequest semantics). Returns (content, usage) or (None, None)."""
        headers = {"Content-Type": "application/json", "User-Agent": "IntelliSim-EXP-H2-harness/1.0",
                   "Authorization": "Bearer " + provider_setting("OPENAI_API_KEY")}
        for attempt in range(self.retries + 1):
            rec = {"tag": tag, "http_attempt": attempt + 1, "sent_at": utc_now()}
            t0 = time.monotonic()
            try:
                r = requests.post(self.url, headers=headers, json=body, timeout=self.timeout)
            except requests.RequestException as e:
                rec.update(outcome="network_error", error=type(e).__name__, elapsed_ms=int((time.monotonic() - t0) * 1000))
                self._record(rec, None)
                if attempt == self.retries:
                    return None, None
                time.sleep(min(2 ** attempt, 30))
                continue
            rec.update(status=r.status_code, elapsed_ms=int((time.monotonic() - t0) * 1000))
            if 200 <= r.status_code < 300:
                data = r.json()
                rec.update(outcome="ok", model=data.get("model"), usage=data.get("usage"),
                           finish_reason=(data.get("choices") or [{}])[0].get("finish_reason"))
                self._record(rec, data)
                content = ((data.get("choices") or [{}])[0].get("message") or {}).get("content") or ""
                return content, data.get("usage") or {}
            rec.update(outcome="http_error", body=r.text[:2000])
            self._record(rec, None)
            if r.status_code == 429 and attempt < self.retries:
                ra = r.headers.get("Retry-After", "")
                time.sleep(float(ra) if ra.replace(".", "", 1).isdigit() else min(2 ** attempt, 30))
            elif r.status_code >= 500 and attempt < self.retries:
                time.sleep(min(2 ** attempt, 30))
            else:
                return None, None
        return None, None

    def _record(self, rec: dict, data: dict | None) -> None:
        idx = len(self.attempts) + 1
        rec["index"] = idx
        self.attempts.append(rec)
        if data is not None:
            write_json(self.log_dir / f"{idx:02d}-{rec['tag']}-response.json", data)

    def call_json(self, body: dict, fields: dict, tag: str, usage: dict) -> dict | None:
        """callJSON: 2 attempts; usage folded only for provider-successful attempts (as the engine)."""
        for i in range(2):
            write_json(self.log_dir / f"req-{tag}-try{i + 1}.json", body)
            content, u = self._http(body, f"{tag}-try{i + 1}")
            if content is None:
                continue
            det = (u.get("completion_tokens_details") or {})
            usage["prompt_tokens"] += u.get("prompt_tokens", 0)
            usage["completion_tokens"] += u.get("completion_tokens", 0)
            usage["reasoning_tokens"] += det.get("reasoning_tokens", 0)
            usage["total_tokens"] += u.get("total_tokens", 0)
            usage["calls"] += 1
            parsed = go_unmarshal(content, fields)
            if parsed is not None:
                return parsed
        return None


def run_report(n: int, c: IntelliSimClient) -> dict:
    ctx = build_context(n, c)
    d = OUT / f"a-{n}"
    caller = Caller(d / "raw")
    usage = {"prompt_tokens": 0, "completion_tokens": 0, "reasoning_tokens": 0, "total_tokens": 0, "calls": 0}
    created, t0 = utc_now(), time.monotonic()
    sections, warnings = [], []
    for key in ctx["plan"]:
        title = GUIDE[key][0]
        system, user = section_prompts(ctx, key)
        out = caller.call_json(request_body(system, user), SECTION_FIELDS, key, usage)
        if out is not None:
            sections.append({"key": key, "title": out["title"] or title, "narrative": out["narrative"],
                             "key_findings": out["key_findings"], "causal_links": out["causal_links"],
                             "emergent_phenomena": out["emergent_phenomena"]})
        else:
            warnings.append({"stage": "section:" + key, "code": "section_failed",
                             "message": "the LLM did not return a valid analysis for this section"})
            sections.append({"key": key, "title": title, "narrative": "This section could not be generated.",
                             "key_findings": [], "causal_links": [], "emergent_phenomena": []})
    report = {"sections": sections, "executive_summary": "", "conclusions": [], "recommendations": []}
    system, user = synthesis_prompts(ctx, sections)
    syn = caller.call_json(request_body(system, user), SYNTH_FIELDS, "synthesis", usage)
    if syn is not None:
        report.update(syn)
    else:
        warnings.append({"stage": "synthesis", "code": "synthesis_failed", "message": "the LLM did not return a valid synthesis"})
    finished = utc_now()
    wall_ms = int((time.monotonic() - t0) * 1000)
    models = sorted({a.get("model") for a in caller.attempts if a.get("model")})
    failed = [a for a in caller.attempts if a["outcome"] != "ok"]
    analysis = {
        "arm": "A", "generator": "harness-replica (harness/exph2_ablate.py)", "evidence_mode": "model_only",
        "report_no": n, "simulation_id": ctx["sim_id"], "name": f"EXP-H2 A report {n}",
        "context_message": ctx["context"], "language": ctx["language"],
        "llm_target": {"provider_uid": "openai", "model_uid": MODEL_UID, "max_completion_tokens": MAX_TOKENS},
        "status": "completed" if (syn is not None or any(s["narrative"] != "This section could not be generated." for s in sections)) else "failed",
        "report": report, "warnings": warnings,
        "token_usage": {"total": usage, "by_model": models},
        "requests": len(ctx["plan"]) + 1, "http_attempts": len(caller.attempts), "failed_attempts": len(failed),
        "created_at": created, "finished_at": finished, "wall_clock_ms": wall_ms,
        "cost_usd_engine_equivalent": round(cost_usd(MODEL_UID, usage["prompt_tokens"], usage["completion_tokens"]), 6),
    }
    write_json(d / "report.json", analysis)
    write_json(d / "attempts.json", caller.attempts)
    return analysis


# ---------------------------------------------------------------- dry run

def dry_run(reports: list[int], c: IntelliSimClient) -> None:
    nfr = verify_against_go_source()
    print(f"go-source check: {nfr} prompt fragments found verbatim")
    banned_keys = ("population_over_time", "property_series", "statistics", "spatial_snapshots", "StepsCompleted",
                   "steps completed", "HEADLINE STATISTICS")
    hashes = {}
    for n in reports:
        ctx = build_context(n, c)
        d = OUT / "dryrun" / f"a-{n}"
        for key in ctx["plan"]:
            system, user = section_prompts(ctx, key)
            body = request_body(system, user)
            assert not any(b in user for b in banned_keys), f"report {n} {key}: run data leaked into prompt"
            p = d / f"req-{key}.json"
            write_json(p, body)
            hashes[f"a-{n}/{key}"] = hashlib.sha256(json.dumps(body, ensure_ascii=False, sort_keys=True).encode("utf-8")).hexdigest()
        (d / "evidence.json").write_text(go_json({"model": ctx["model"], "run": ctx["run"]}), encoding="utf-8")
        print(f"a-{n}: plan={ctx['plan']} run={ctx['run']['Name']} desc={ctx['model']['Description'][:70]!r}")
    write_json(OUT / "dryrun" / "a-hashes.json", hashes)


def main() -> int:
    console_utf8()
    ap = argparse.ArgumentParser()
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--dry-run", action="store_true")
    g.add_argument("--run", action="store_true")
    ap.add_argument("--reports", default="1-10")
    ap.add_argument("--report", type=int)
    a = ap.parse_args()
    c = IntelliSimClient()
    if a.dry_run:
        lo, _, hi = a.reports.partition("-")
        dry_run(list(range(int(lo), int(hi or lo) + 1)), c)
        return 0
    verify_against_go_source()
    res = run_report(a.report, c)
    print(json.dumps({k: res[k] for k in ("report_no", "status", "http_attempts", "failed_attempts", "wall_clock_ms",
                                          "cost_usd_engine_equivalent")}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
