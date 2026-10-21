"""Shared helpers for EXP-H2 (D24): paths, prices, Go-compatible JSON, provider key.

EXP-H2 = two arms over the 10 primary EXP-H runs:
  F2 — full regeneration through the unchanged IntelliSim engine API (exph2_regen.py)
  A  — model-only ablation via a standalone replica of the engine's Level-3 prompts
       that calls the OpenAI Chat Completions API directly (exph2_ablate.py)
Protocol: experiments/37-exph2-regeneration-ablation-protocol.md.
"""

from __future__ import annotations

import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
WORKSPACE = REPO.parent
SIBLING_HARNESS = WORKSPACE / "intellisim.experiments" / "harness"
PROVIDER_ENV = WORKSPACE / "intellisim.openai.llm.provider" / ".env"
EXPH = REPO / "results" / "exp-h"
OUT = REPO / "results" / "exp-h2"

PRIMARY = list(range(1, 11))
MODEL_UID = "o4-mini"
MAX_TOKENS = 16_000                       # cost-only cap (D24); passed as max_completion_tokens
GENERATION_CAP = {"max_tokens": {"uint": MAX_TOKENS}}   # engine ConfigValueDTO shape (GET /llms)
PRICE_PER_1M = {"o4-mini": (1.10, 4.40), "o3": (2.00, 8.00)}   # list, re-verified 2026-09-23

# Gates (protocol 37 §7)
G2_MAX_COST_PER_REPORT = 0.06
G3_MAX_CUMULATIVE_COST = 1.20
G3_MAX_FAILED_ATTEMPTS = 2


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def load_primary(n: int) -> tuple[dict, dict]:
    """Return (request.json, original analysis object) of EXP-H report n."""
    req = json.loads((EXPH / f"report-{n}" / "request.json").read_text(encoding="utf-8"))
    rep = json.loads((EXPH / f"report-{n}" / "report.json").read_text(encoding="utf-8"))
    return req, rep.get("analysis", rep)


def go_json(obj) -> str:
    """Byte-compatible with Go encoding/json.Marshal for the value kinds used here
    (ordered dicts, lists, str, int, bool, None): compact separators, UTF-8 kept,
    HTML-sensitive <, >, & and U+2028/U+2029 escaped as Go does."""
    s = json.dumps(obj, ensure_ascii=False, separators=(",", ":"))
    return (s.replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
             .replace("\u2028", "\\u2028").replace("\u2029", "\\u2029"))


def cost_usd(model: str, prompt_tokens: int, completion_tokens: int) -> float:
    base = "o3" if model.startswith("o3") else "o4-mini"
    pin, pout = PRICE_PER_1M[base]
    return prompt_tokens / 1e6 * pin + completion_tokens / 1e6 * pout


def provider_setting(name: str) -> str:
    """Read one `$env:NAME="value"` assignment from the provider's PowerShell-format
    .env (last assignment wins, as with Invoke-Expression). Values are never printed."""
    text = PROVIDER_ENV.read_text(encoding="utf-8")
    vals = re.findall(r'\$env:' + re.escape(name) + r'\s*=\s*"([^"]*)"', text)
    if not vals:
        raise RuntimeError(f"{name} not found in provider .env")
    return vals[-1]


def write_json(path: Path, obj) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(obj, ensure_ascii=False, indent=2), encoding="utf-8")


def console_utf8() -> None:
    sys.stdout.reconfigure(encoding="utf-8")
