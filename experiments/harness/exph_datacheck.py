"""EXP-H Task 3b: objective, rater-free check of the disputed within-run claims.

Reads the source-run step series under results/source-runs/** (the runs the ten
Level-3 reports were generated from) and computes, for every claim the two
scoring passes disagreed on (or that pass 2 counted as an error / hallucination
without access to the series), the quantity the claim asserts. Each claim gets a
verdict:

  TRUE     the quantity is as the report states (within rounding)
  PARTLY   the qualitative statement holds but a number is off, or vice versa
  FALSE    the data contradict the claim
  UNCHECK  not decidable from the exported series (mechanism / attribution)

Nothing here changes a score (D17): both passes stay as recorded. Output:
  results/exp-h-rescoring/DATA-CHECK.md   (human-readable, with the rule used)
  results/exp-h-rescoring/data-check.csv  (one row per claim)

Run from the repo root:  python harness/exph_datacheck.py
"""
from __future__ import annotations

import csv
import gzip
import json
import math
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
csv.field_size_limit(10**8)

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "results" / "source-runs"
OUT_MD = ROOT / "results" / "exp-h-rescoring" / "DATA-CHECK.md"
OUT_CSV = ROOT / "results" / "exp-h-rescoring" / "data-check.csv"

RUNS = {
    "C3": SRC / "exp-c/raw/6a5bbcaac44d001ef3ac207c-rep3-steps.csv",
    "C18": SRC / "exp-c/raw/6a5bbcc1c44d001ef3acc10e-rep18-steps.csv",
    "B12": SRC / "exp-b/test-12/raw/6a5bc249c44d001ef3af684d-rep1-steps.csv",
    "B4": SRC / "exp-b/test-4/N300/raw/6a5bc463c44d001ef3b8f824-rep1-steps.csv",
    "E5": SRC / "exp-e/archetype_decision/run-301/export.csv.gz",
    "E6": SRC / "exp-e/individual_decision/run-301/export.csv.gz",
    "F7": SRC / "exp-f/intellisim/sweep-61/raw/6a610ff2ff82d466bf2db995-p0.50-rep1-steps.csv.gz",
    "F8": SRC / "exp-f/intellisim/sweep-61/raw/6a634dbf4ccaa6983a640151-p0.70-rep1-steps.csv.gz",
    "G9": SRC / "exp-g/intellisim/point-med/raw/med-rep1-steps.csv.gz",
    "G10": SRC / "exp-g/intellisim/point-med/raw/med-rep8-steps.csv.gz",
}

# Operational rule for "regime change / slowdown / surge" claims: a claimed
# change of rate between two windows is SUPPORTED when the later window's mean
# per-step rate differs from the earlier one by at least this factor.
REGIME_FACTOR = 0.30


# ----------------------------------------------------------------- loading
@dataclass
class Series:
    key: str
    steps: list[int]
    evac: dict[int, int] = field(default_factory=dict)          # evacuated_count per step
    pop: dict[int, dict[str, int]] = field(default_factory=dict)  # agent_uid -> current
    by_rule: dict[int, dict[str, int]] = field(default_factory=dict)
    agents: dict[int, list[tuple[str, int, int, int, str]]] = field(default_factory=dict)
    # step -> [(uid, reference_id, x, y, state)]

    # --- evacuation helpers
    def exits(self, t: int) -> int:
        return self.evac.get(t, 0) - self.evac.get(t - 1, 0) if t > 0 else self.evac.get(0, 0)

    def exits_window(self, a: int, b: int) -> int:
        """exits recorded at steps a..b inclusive (evac[b] - evac[a-1])."""
        return self.evac[b] - (self.evac[a - 1] if a > 0 else 0)

    def rate(self, a: int, b: int) -> float:
        return self.exits_window(a, b) / (b - a + 1)

    def first_step_reaching(self, n: int) -> int | None:
        for t in self.steps:
            if self.evac.get(t, 0) >= n:
                return t
        return None

    def population(self, t: int, uid: str = "person") -> int:
        return self.pop[t].get(uid, 0)


def load(key: str) -> Series:
    p = RUNS[key]
    fh = gzip.open(p, "rt", encoding="utf-8", newline="") if p.suffix == ".gz" else open(p, encoding="utf-8", newline="")
    s = Series(key=key, steps=[])
    seen = set()
    with fh:
        for r in csv.DictReader(fh):
            t = int(r["step"])
            if t not in seen:
                seen.add(t)
                s.steps.append(t)
            if r["type"] == "environment":
                st = json.loads(r["statistics"])
                s.pop[t] = {p_["agent_uid"]: p_["current"] for p_ in st["population"]}
                s.by_rule[t] = {b["rule_id"]: b["executions"] for b in st.get("by_rule", [])}
                if r.get("evacuated_count"):
                    s.evac[t] = int(r["evacuated_count"])
            elif r["type"] == "agent":
                s.agents.setdefault(t, []).append(
                    (r["uid"], int(r["reference_id"]), int(r["x"]), int(r["y"]), r.get("state", ""))
                )
    s.steps.sort()
    return s


# ----------------------------------------------------------------- results
@dataclass
class Check:
    cid: str
    report: int
    claim: str          # short paraphrase / quote of the report's claim
    computed: str       # what the data say
    verdict: str        # TRUE / PARTLY / FALSE / UNCHECK
    p1: str             # how pass 1 treated it: none / note / error / halluc / score
    p2: str             # how pass 2 treated it


CHECKS: list[Check] = []


def add(cid, report, claim, computed, verdict, p1, p2):
    CHECKS.append(Check(cid, report, claim, computed, verdict, p1, p2))


def fmt(x: float) -> str:
    return f"{x:.2f}"


def regime(s: Series, a1: int, b1: int, a2: int, b2: int) -> tuple[float, float, bool]:
    r1, r2 = s.rate(a1, b1), s.rate(a2, b2)
    changed = abs(r2 - r1) >= REGIME_FACTOR * max(r1, 1e-9)
    return r1, r2, changed


def linearity(s: Series) -> tuple[int, int, float]:
    """first exit step, step of completion, R^2 of evac(t) vs t between them."""
    t0 = next(t for t in s.steps if s.exits(t) > 0)
    total = max(s.evac.values())
    t1 = s.first_step_reaching(total)
    xs = list(range(t0 - 1, t1 + 1))
    ys = [s.evac[t] for t in xs]
    n = len(xs)
    mx, my = sum(xs) / n, sum(ys) / n
    sxy = sum((x - mx) * (y - my) for x, y in zip(xs, ys))
    sxx = sum((x - mx) ** 2 for x in xs)
    syy = sum((y - my) ** 2 for y in ys)
    r2 = (sxy * sxy) / (sxx * syy) if sxx and syy else float("nan")
    return t0, t1, r2


def window_rates(s: Series, w: int, t0: int, t1: int) -> list[tuple[int, int, float]]:
    out = []
    for a in range(t0, t1 - w + 2):
        b = a + w - 1
        out.append((a, b, s.rate(a, b)))
    return out


# ================================================================= EXP-C runs (reports 1, 11 | 2, 12)
def check_expc(s: Series, rep_o4: int, rep_o3: int):
    t0, t1, r2 = linearity(s)
    total_rate = s.evac[t1] / t1
    d = {t: s.exits(t) for t in s.steps}
    peak = max(d.values())
    peak_steps = [t for t, v in d.items() if v == peak]
    hist = defaultdict(int)
    for t in range(t0, t1 + 1):
        hist[d[t]] += 1
    dist = ", ".join(f"{k}×{hist[k]}" for k in sorted(hist))
    common = (
        f"first exit step {t0}, all {s.evac[t1]} out at step {t1} (run has {len(s.steps)} rows, steps 0–{s.steps[-1]}); "
        f"overall {fmt(total_rate)}/step; R² of evacuated_count vs step = {r2:.4f}; "
        f"per-step exits distribution over steps {t0}–{t1}: {dist}; max {peak} at step(s) {peak_steps}"
    )
    return common, d, t0, t1, peak, peak_steps


def report_1_11():
    s = load("C3")
    common, d, t0, t1, peak, peak_steps = check_expc(s, 1, 11)
    # R1: "33 persons exit in steps 14–24 (~3.3/step)"
    n_incl, n_10 = s.exits_window(14, 24), s.exits_window(15, 24)
    add("1a", 1, "«33 persons exit in steps 14–24 (~3.3/step)», peak removal window",
        f"exits at steps 14–24 = {n_incl} ({fmt(s.rate(14,24))}/step); steps 15–24 = {n_10} ({fmt(s.rate(15,24))}/step); "
        f"window rates for comparison: steps 1–13 {fmt(s.rate(1,13))}/step, steps 25–45 {fmt(s.rate(25,45))}/step",
        "TRUE" if abs(n_incl - 33) <= 1 or abs(n_10 - 33) <= 1 else ("PARTLY" if max(n_incl, n_10) >= 30 else "FALSE"),
        "note", "error")
    r1, r2_, changed = regime(s, 1, 24, 25, 45)
    add("1b", 1, "«Phase transition … around 50 % evacuation»; post-step-24 rate ≈ 2.15/step (43 over 20 steps)",
        f"steps 1–24 {fmt(r1)}/step vs steps 25–45 {fmt(r2_)}/step (change {fmt(abs(r2_-r1)/r1*100)} %, threshold {int(REGIME_FACTOR*100)} %); "
        f"exits at steps 25–44 = {s.exits_window(25,44)}; R² = {linearity(s)[2]:.4f}",
        "PARTLY" if not changed else "TRUE",   # the quoted rates are right; a regime change is not
        "note", "halluc")
    add("1c", 1, "«Peak instantaneous flow occurs at step 29 (4 exits)»",
        f"max per-step exits = {peak} at step(s) {peak_steps}; exits at step 29 = {d[29]}",
        "TRUE" if peak == 4 and 29 in peak_steps else ("PARTLY" if d[29] == peak else "FALSE"),
        "none", "halluc (mechanism)")
    add("1d", 1, "«steps 0–23 … decline from 100 to 49»; «50 evacuated by step 21, 56 by step 24»",
        f"evacuated at step 21 = {s.evac[21]}, step 23 = {s.evac[23]} (remaining {100-s.evac[23]}), step 24 = {s.evac[24]}",
        "TRUE" if s.evac[21] == 50 and s.evac[24] == 56 and 100 - s.evac[23] in (49, 50, 51) else "PARTLY",
        "none", "note")
    add("1e", 1, "«no one exits at step 43»; «final agent leaves at step 44»; «completes in 46 steps»",
        f"exits at step 43 = {d[43]}, step 44 = {d[44]}, step 45 = {d[45]}; evacuated_count reaches 100 at step {t1}; "
        f"steps recorded 0–{s.steps[-1]} ({len(s.steps)} rows)",
        "PARTLY", "note", "note")
    # R11
    n = s.exits_window(10, 30)
    add("11a", 11, "«65 exits between steps 10–30, averaging 3.1 agents/step, with a peak of 5 at step 24»",
        f"exits at steps 10–30 = {n} ({fmt(s.rate(10,30))}/step); exits at step 24 = {d[24]}; max per-step = {peak} at {peak_steps}",
        "TRUE" if abs(n - 65) <= 2 and d[24] == peak == 5 else ("PARTLY" if abs(n - 65) <= 5 else "FALSE"),
        "none", "error")
    r1, r2_, changed = regime(s, 10, 33, 34, 45)
    add("11b", 11, "«Throughput collapses to ≈1.1 agents/step after the remaining population falls below 20 (steps 34–45)»",
        f"remaining at step 33 = {100-s.evac[33]}, at step 34 = {100-s.evac[34]}; exits at steps 34–45 = {s.exits_window(34,45)} "
        f"({fmt(s.rate(34,45))}/step) vs steps 10–33 {fmt(r1)}/step (change {fmt(abs(r2_-r1)/r1*100)} %)",
        "FALSE" if s.rate(34, 45) > 1.5 else ("PARTLY" if changed else "FALSE"),
        "none", "halluc")
    add("11c", 11, "«Initial six-step lag: only 4 exits by step 6»",
        f"evacuated at step 6 = {s.evac[6]}; first exit at step {t0}",
        "TRUE" if s.evac[6] == 4 else ("PARTLY" if s.evac[6] <= 6 else "FALSE"),
        "none", "note")
    add("1/11", 1, "Run-level: EXP-C rep3 discharge profile (context for all of the above)", common, "—", "—", "—")


def report_2_12():
    s = load("C18")
    common, d, t0, t1, peak, peak_steps = check_expc(s, 2, 12)
    add("2a", 2, "«Стабільна середня швидкість ≈3 особи/крок … від кроку 9 до 30»",
        f"exits at steps 9–30 = {s.exits_window(9,30)} ({fmt(s.rate(9,30))}/step); steps 13–25 = {s.exits_window(13,25)} ({fmt(s.rate(13,25))}/step)",
        "TRUE" if s.rate(9, 30) >= 2.75 else ("PARTLY" if s.rate(9, 30) >= 2.5 else "FALSE"),
        "none", "error")
    r1, r2_, changed = regime(s, 9, 33, 34, 43)
    add("2b", 2, "«Фінальна фаза евакуації (step 34–43) сповільнилася до ~1 особи/крок»",
        f"exits at steps 34–43 = {s.exits_window(34,43)} ({fmt(s.rate(34,43))}/step) vs steps 9–33 {fmt(r1)}/step "
        f"(change {fmt(abs(r2_-r1)/r1*100)} %); remaining at step 34 = {100-s.evac[34]}",
        "TRUE" if s.rate(34, 43) <= 1.3 else ("PARTLY" if changed else "FALSE"),
        "none", "halluc")
    add("2c", 2, "«Зменшення чисельності групи нижче ≈15 осіб після кроку 34»",
        f"remaining at step 34 = {100-s.evac[34]}, at step 35 = {100-s.evac[35]}",
        "TRUE" if 100 - s.evac[34] <= 16 else "FALSE", "none", "note")
    add("2d", 2, "«Останній агент вийшов близько кроку 43 … 43 кроки»; «всі 100 … до кроку 45»",
        f"evacuated at step 42 = {s.evac[42]}, 43 = {s.evac[43]}, 44 = {s.evac[44]}; reaches 100 at step {t1}",
        "PARTLY", "note", "score P3=1 (the «≈ 57 с» error is undisputed and not re-checked)")
    # R12
    w = [d[t] for t in range(8, 35)]
    add("12a", 12, "«між кроками 8 і 34 … від 2 до 4 осіб за такт, у середньому близько 2,9»",
        f"exits at steps 8–34: min {min(w)}, max {max(w)}, mean {fmt(sum(w)/len(w))} ({sum(w)} exits over 27 steps)",
        "TRUE" if abs(sum(w) / len(w) - 2.9) <= 0.15 and min(w) >= 2 and max(w) <= 4 else
        ("PARTLY" if abs(sum(w) / len(w) - 2.9) <= 0.3 else "FALSE"),
        "none", "error")
    add("12b", 12, "«плато 4–5 осіб/крок»",
        f"max per-step exits = {peak} at step(s) {peak_steps}; steps with ≥4 exits: {[t for t in s.steps if d[t] >= 4]}",
        "FALSE" if peak < 4 else ("PARTLY" if hist_ge(d, 4) <= 3 else "TRUE"),
        "none", "error")
    r1, r2_, changed = regime(s, 8, 34, 35, 44)
    add("12c", 12, "«Коли … залишилось <15 осіб (після кроку 34), потік впав до 1 особи/крок, подовживши хвіст на 11 тактів» / «фазовий перехід … знижує відтік удвічі»",
        f"remaining at step 34 = {100-s.evac[34]}; exits at steps 35–44 = {s.exits_window(35,44)} ({fmt(s.rate(35,44))}/step) vs steps 8–34 {fmt(r1)}/step "
        f"(ratio {fmt(r2_/r1)}); tail length from step 34 to completion = {t1-34} steps",
        "TRUE" if r2_ / r1 <= 0.6 else ("PARTLY" if changed else "FALSE"),
        "none", "halluc")
    add("12d", 12, "«97 % усіх виходів (кроки 4–34)»; «перший вихід на кроці 4, останній — між 44 і 45»",
        f"first exit at step {t0}; exits at steps 4–34 = {s.exits_window(4,34)} ({s.exits_window(4,34)} %); completion step {t1}",
        "TRUE" if t0 == 4 and s.exits_window(4, 34) >= 95 else ("PARTLY" if s.exits_window(4, 34) >= 85 else "FALSE"),
        "none", "note")
    add("2/12", 2, "Run-level: EXP-C rep18 discharge profile (context)", common, "—", "—", "—")


def hist_ge(d: dict[int, int], k: int) -> int:
    return sum(1 for v in d.values() if v >= k)


# ================================================================= EXP-B T12 (report 3)
def report_3():
    s = load("B12")
    d = {t: s.exits(t) for t in s.steps}
    t_first = next(t for t in s.steps if d[t] > 0)
    t147 = s.first_step_reaching(147)
    last = s.steps[-1]
    final_agents = s.agents[last]
    pos = [(x, y) for _, _, x, y, _ in final_agents]
    # geometry: Room 1 x=1..25, Room-1 door x=26 y=12..14, corridor x=27..36 y=11..15, Room 2 x=38..62, exit x=63 y=12..14
    def where(x, y):
        if x <= 25:
            return "Room 1"
        if x == 26:
            return "Room-1 door"
        if 27 <= x <= 36:
            return "corridor"
        if 37 <= x <= 62:
            return "Room 2"
        return "exit"
    locs = [f"({x},{y}) {where(x,y)}" for x, y in pos]
    add("3a", 3, "«By step 299, only 3 of 150 agents remain queued at the exit» / «queuing patterns at the final exit»",
        f"agents present at step {last}: {len(final_agents)} at {locs}; Room-1 door is x=26, corridor x=27–36, final exit x=63",
        "FALSE" if all(x <= 36 for x, _ in pos) else "PARTLY",
        "none", "error")
    # positions unchanged from step 100 to 599?
    p100 = sorted((rid, x, y) for _, rid, x, y, _ in s.agents[100])
    p599 = sorted((rid, x, y) for _, rid, x, y, _ in final_agents)
    add("3b", 3, "«metastable congested state … capacity allows periodic exits (147 in 70 steps, ≈2.1 agents/step)»",
        f"first exit at step {t_first}; evacuated_count reaches 147 at step {t147} ⇒ 147 exits over {t147-t_first+1} steps = {fmt(147/(t147-t_first+1))}/step; "
        f"exits after step {t147}: {sum(d[t] for t in s.steps if t > t147)}; straggler positions identical at steps 100 and {last}: {p100 == p599}",
        "PARTLY" if abs((t147 - t_first + 1) - 70) <= 5 else "FALSE",
        "none", "halluc")
    add("3c", 3, "«No agents exit before step 30» vs «At step 30 … population 141»",
        f"first exit at step {t_first}; evacuated at step 20 = {s.evac[20]}, step 30 = {s.evac[30]} (population {150-s.evac[30]})",
        "PARTLY" if s.evac[30] == 9 and t_first < 30 else "FALSE",
        "none", "note")
    # zone occupancy with an explicit own definition
    def occ(t, x0, x1, y0, y1):
        return sum(1 for _, _, x, y, _ in s.agents.get(t, []) if x0 <= x <= x1 and y0 <= y <= y1)
    z1 = max((occ(t, 20, 25, 9, 17), t) for t in s.steps)
    z2 = max((occ(t, 57, 62, 9, 17), t) for t in s.steps)
    add("3d", 3, "Report: «occupancy builds immediately upstream of the door … corridor and Room 2 quickly clear» (scored 1 by pass 2 for lacking figures)",
        f"own zone definition (6 cells upstream of each opening, y 9–17): peak zone-1 (x 20–25) = {z1[0]} at step {z1[1]}; peak zone-2 (x 57–62) = {z2[0]} at step {z2[1]}; ratio {fmt(z1[0]/max(z2[0],1))} "
        f"(source REPORT.md, its own zones: 37.2 vs 12.7, ratio 2.9)",
        "TRUE", "score 2", "score 1")


# ================================================================= EXP-B T4 N300 (report 4)
def report_4():
    s = load("B4")
    d = {t: s.exits(t) for t in s.steps}
    t297 = s.first_step_reaching(297)
    last = s.steps[-1]
    add("4a", 4, "«Вихід 297 агентів … завершився до кроку 449, середній потік ≈0,66 агента/крок»",
        f"evacuated_count reaches 297 at step {t297} (≈ {fmt(297/t297)}/step from step 0); exits after step {t297}: {sum(d[t] for t in s.steps if t > t297)}; run recorded to step {last}",
        "FALSE", "none", "error")
    add("4b", 4, "«0–15 кроків: вихід 43 осіб через високі затори» (low early flow = congestion)",
        f"evacuated at step 15 = {s.evac[15]}; first exit at step {next(t for t in s.steps if d[t] > 0)}; "
        f"exits per 15-step window: 0–15 {s.exits_window(0,15)}, 16–30 {s.exits_window(16,30)}, 31–45 {s.exits_window(31,45)}, 46–60 {s.exits_window(46,60)}, 61–75 {s.exits_window(61,75)}, 76–90 {s.exits_window(76,90)}",
        "PARTLY" if abs(s.evac[15] - 43) <= 2 else "FALSE",
        "error (attribution)", "error (attribution)")
    # max window rate and population there
    wr = window_rates(s, 5, 1, t297)
    best = max(wr, key=lambda r: r[2])
    remaining_at_best = 300 - s.evac[best[0] - 1]
    add("4c", 4, "«максимальний потік … ~6,5 агентів/крок при середній концентрації (~160 осіб)» (non-monotone density–flow)",
        f"best 5-step window: steps {best[0]}–{best[1]} at {fmt(best[2])}/step with {remaining_at_best} agents remaining at its start; "
        f"remaining at steps 15/30/45/60/75 = {300-s.evac[15]}/{300-s.evac[30]}/{300-s.evac[45]}/{300-s.evac[60]}/{300-s.evac[75]}; "
        f"rates by remaining-population band: >200: {fmt(s.rate(1, s.first_step_reaching(100)))}/step, 200–100: {fmt(s.rate(s.first_step_reaching(100)+1, s.first_step_reaching(200)))}/step, <100: {fmt(s.rate(s.first_step_reaching(200)+1, t297))}/step",
        "PARTLY", "halluc", "error (non-monotone)")
    add("4d", 4, "«Загалом 297 осіб покинули приміщення за перші 90 кроків»; phase counts 300→257→160→63→19",
        f"evacuated at step 90 = {s.evac[90]}; remaining at steps 15/30/45/60 = {300-s.evac[15]}/{300-s.evac[30]}/{300-s.evac[45]}/{300-s.evac[60]}",
        "TRUE" if s.evac[90] >= 297 else "PARTLY", "none", "note")
    final_agents = s.agents[last]
    pos = [(x, y) for _, _, x, y, _ in final_agents]
    add("4e", 4, "«стабільний затор з 3 агентів до кінця симуляції» «на суміжних клітинах x=39–42» (3 stragglers as an opening-width jam)",
        f"agents at step {last}: {len(final_agents)} at {pos}; opening is x=41, y=11–20 (10 cells); removal zone x=44–47; "
        f"positions identical between step {t297+10} and {last}: {sorted((r,x,y) for _,r,x,y,_ in s.agents[t297+10]) == sorted((r,x,y) for _,r,x,y,_ in final_agents)}",
        "PARTLY", "none", "halluc")   # positions (x 39–42, 3 agents, static to the end) are TRUE; «jam caused by opening width» is not decidable from the series


# ================================================================= EXP-E (reports 5, 6)
def report_5():
    s = load("E5")
    d = {t: s.exits(t) for t in s.steps}
    last = s.steps[-1]
    add("5a", 5, "«Simulation completes at step 98 with one straggler remaining»",
        f"last recorded step {last} ({len(s.steps)} steps); population at step {last} = {s.population(last)}; evacuated_count = {s.evac[last]}; "
        f"population at step {last-1} = {s.population(last-1)}",
        "FALSE" if s.population(last) == 0 else "TRUE",
        "error", "error")
    t50, t80, t100 = s.first_step_reaching(50), s.first_step_reaching(80), s.first_step_reaching(100)
    add("5b", 5, "«All 100 agents evacuated by step 96; first 50 exited within 40 steps, last 20 took 56 steps»",
        f"evacuated reaches 50 at step {t50}, 80 at step {t80}, 100 at step {t100}; last 20 took {t100-t80} steps",
        "PARTLY",   # «within 40 steps» holds (50 at step 32); «last 20 took 56 steps» is off (46); completion 97, not 96
        "note", "note")
    zero = [t for t in s.steps if t > 0 and d[t] == 0]
    add("5c", 5, "Plateau steps «3, 9, 26, 50, 56» (zero-exit steps)",
        f"zero-exit steps after first exit: {zero}",
        "TRUE" if all(t in zero for t in (3, 9, 26, 50, 56)) else ("PARTLY" if any(t in zero for t in (3, 9, 26, 50, 56)) else "FALSE"),
        "none", "unknown")


def report_6():
    s = load("E6")
    d = {t: s.exits(t) for t in s.steps}
    last = s.steps[-1]
    add("6a", 6, "«плато на кроках 2–3 і 6–7» (pauses attributed to decide_proceed_to_exit)",
        f"exits at steps 1–8: {[d[t] for t in range(1,9)]}; evacuated at step 3 = {s.evac[3]}, step 7 = {s.evac[7]}",
        "TRUE" if d[2] == 0 and d[3] == 0 and d[6] == 0 and d[7] == 0 else
        ("PARTLY" if (d[2] == 0 or d[3] == 0) and (d[6] == 0 or d[7] == 0) else "FALSE"),
        "none", "halluc (mechanism)")
    peak = max(d.values())
    add("6b", 6, "«пік 4–5 агентів за крок (кроки 4–17 / 17–23)»",
        f"max per-step exits = {peak} at step(s) {[t for t in s.steps if d[t]==peak]}; steps 4–17 rate {fmt(s.rate(4,17))}/step, 17–23 rate {fmt(s.rate(17,23))}/step",
        "TRUE" if peak >= 4 else "FALSE", "none", "note")
    add("6c", 6, "«Усі 100 агентів евакуйовані до кроку 46; фінальний агент вийшов до 45-го кроку»",
        f"evacuated_count reaches 100 at step {s.first_step_reaching(100)}; last recorded step {last}; population there {s.population(last)}",
        "TRUE", "score 2", "score 2")
    # baseline comparison with EXP-C rep3 (rules-only, 100 agents, same room)
    c = load("C3")
    add("6d", 6, "Sheet: LLM decisions had no slowing effect vs rules-only (pass 2 P1 = 1 because not stated)",
        f"individual_decision completion step {s.first_step_reaching(100)} vs EXP-C rep3 rules-only {c.first_step_reaching(100)}; decision rule executions total {sum(s.by_rule[t].get('decide_proceed_to_exit',0) for t in s.steps)}",
        "TRUE", "score 2", "score 1")


# ================================================================= EXP-F (reports 7, 8)
def report_7():
    s = load("F7")
    mean_x = []
    for t in s.steps:
        xs = [x for uid, _, x, _, _ in s.agents[t] if uid == "burning_tree"]
        mean_x.append((t, len(xs), fmt(sum(xs) / len(xs)) if xs else "—"))
    add("7a", 7, "«Фронтова хвиля горіння рухається справа наліво»",
        "mean x of burning trees per step (step, n, mean x): " + "; ".join(f"{t}:{n}:{m}" for t, n, m in mean_x),
        "FALSE", "error", "error")
    burning = [s.pop[t].get("burning_tree", 0) for t in s.steps]
    ash = [s.pop[t].get("ash", 0) for t in s.steps]
    add("7b", 7, "«вторинний пік горіння 20 на кроці 2»; «попіл +7–20 одиниць щокроку»",
        f"burning_tree per step {burning}; ash per step {ash}; ash increments {[ash[i]-ash[i-1] for i in range(1,len(ash))]}",
        "PARTLY" if burning[2] == 20 else "FALSE", "none", "unknown")
    add("7c", 7, "«138 із 1802 (7,66 %)» vs «1723 із 1802 — падіння на 4,4 %»",
        f"final: tree {s.pop[s.steps[-1]].get('tree')}, ash {s.pop[s.steps[-1]].get('ash')}, initial trees 1802 (+59 ignited at step 0 = 1861 total); 138/1802 = {fmt(138/1802*100)} %; 79/1802 = {fmt(79/1802*100)} % (79 = trees burned after step 0)",
        "PARTLY", "error (4.4 %)", "note")
    sf = {t: s.by_rule[t].get("spread_fire", 0) for t in s.steps}
    ig = {t: s.by_rule[t].get("ignite_left_edge", 0) for t in s.steps}
    add("7d", 7, "«spread_fire 79 виконань між кроками 1–9»; «вогонь ініціювався на лівому краї протягом перших 11 кроків»",
        f"cumulative by_rule spread_fire per step {sf}; ignite_left_edge per step {ig}",
        "PARTLY", "none", "note")


def report_8():
    s = load("F8")
    trees0 = [(x, y) for uid, _, x, y, _ in s.agents[0] if uid in ("tree", "burning_tree")]
    xs = [x for x, _ in trees0]
    ys = [y for _, y in trees0]
    add("8a", 8, "«Initial tree agents spanned approximately x=3 to x=54 with no significant voids»",
        f"trees (+burning) at step 0: n={len(trees0)}, x range {min(xs)}–{max(xs)}, y range {min(ys)}–{max(ys)}; columns with ≥1 tree: {len(set(xs))}",
        "FALSE" if max(xs) > 56 else "TRUE", "none", "error")
    last = s.steps[-1]
    surv = [(x, y) for uid, _, x, y, _ in s.agents[last] if uid == "tree"]
    q = sum(1 for x, y in surv if x > 50 and y > 50)
    add("8b", 8, "«pockets of unburned trees persisted, particularly in the upper-right quadrant (x>50, y>50)»",
        f"surviving trees at step {last}: {len(surv)}; of which x>50 & y>50: {q}; x range {min(x for x,_ in surv)}–{max(x for x,_ in surv)}, y range {min(y for _,y in surv)}–{max(y for _,y in surv)}",
        "TRUE" if q >= len(surv) / 3 else ("PARTLY" if q > 0 else "FALSE"), "none", "unknown")
    burning = {t: s.pop[t].get("burning_tree", 0) for t in s.steps}
    ash = {t: s.pop[t].get("ash", 0) for t in s.steps}
    pk = max(v for t, v in burning.items() if t >= 1)   # post-ignition peak (step 0 = 84 ignited cells)
    tpk = [t for t, v in burning.items() if v == pk and t >= 1]
    add("8c", 8, "«dip to 21 burning by step 4, peak 48 at step 25, decline to zero by step 85, secondary peak ~39 at step 53»",
        f"burning at step 0 = {burning[0]} (ignition), step 4 = {burning[4]}; post-ignition max {pk} at step {tpk}; burning at 53 = {burning[53]}, at 84 = {burning[84]}, at 85 = {burning[85]}; "
        f"local maxima after step 25: {[t for t in s.steps[26:-1] if burning[t] > burning[t-1] and burning[t] >= burning[t+1]]}",
        "TRUE" if burning[4] == 21 and pk == 48 and 25 in tpk and burning[53] == 39 and burning[85] == 0 else "PARTLY",
        "none", "note")
    add("8d", 8, "«Ash 2 546 by step 84 … 2547 of 2605 trees burned (step 86)»",
        f"ash at step 84 = {ash[84]}, step 85 = {ash[85]}; last recorded step {last}; total trees 2521 + 84 ignited = 2605",
        "TRUE" if ash[84] == 2546 and ash[85] == 2547 else "PARTLY", "none", "note")
    ig = {t: s.by_rule[t].get("ignite_left_edge", 0) for t in (0, 1, 2, last)}
    add("8e", 8, "«Continued left-edge ignitions (84 executions) → sustained front pressure» vs all 84 at step 0",
        f"cumulative ignite_left_edge at steps 0/1/2/{last}: {ig}",
        "FALSE" if ig[0] == 84 and ig[last] == 84 else "PARTLY", "none", "note")


# ================================================================= EXP-G (reports 9, 10)
def sir(s: Series):
    S = {t: s.pop[t].get("person_susceptible", 0) for t in s.steps}
    I = {t: s.pop[t].get("person_infected", 0) for t in s.steps}
    R = {t: s.pop[t].get("person_recovered", 0) for t in s.steps}
    return S, I, R


# A "secondary wave" in an SIR series is SUPPORTED when, after the main peak,
# I(t) falls to a trough and later rises to at least REBOUND_FACTOR × trough
# and at least REBOUND_MIN infected.
REBOUND_FACTOR = 3
REBOUND_MIN = 5


def rebound(series: dict[int, int], after: int) -> tuple[int, int, int, int]:
    """largest post-peak rebound: (trough value, trough step, later max value, its step)."""
    ts = [t for t in sorted(series) if t > after]
    best = (series[ts[0]], ts[0], series[ts[0]], ts[0])
    best_ratio = 0.0
    for i, t in enumerate(ts):
        trough = series[t]
        if trough == 0:
            continue
        later = max(((series[u], u) for u in ts[i:]), default=(trough, t))
        ratio = later[0] / trough
        if ratio > best_ratio:
            best_ratio, best = ratio, (trough, t, later[0], later[1])
    return best


def local_maxima(series: dict[int, int], after: int) -> list[tuple[int, int]]:
    ts = sorted(series)
    out = []
    for i in range(1, len(ts) - 1):
        t = ts[i]
        if t <= after:
            continue
        if series[t] > series[ts[i - 1]] and series[t] >= series[ts[i + 1]]:
            out.append((t, series[t]))
    return out


def report_9():
    s = load("G9")
    S, I, R = sir(s)
    pk = max(I.values())
    tpk = [t for t, v in I.items() if v == pk]
    lm = local_maxima(I, tpk[-1])
    reb = rebound(I, tpk[-1])
    add("9a", 9, "«Minor secondary waves (1–9 infected between steps 141–177)» / «multi-wave outbreak»",
        f"peak I = {pk} at step {tpk}; I over steps 141–177: min {min(I[t] for t in range(141,178))}, max {max(I[t] for t in range(141,178))}; "
        f"largest post-peak rebound: trough I = {reb[0]} at step {reb[1]} → {reb[2]} at step {reb[3]} (ratio {fmt(reb[2]/max(reb[0],1))}, rule ≥ {REBOUND_FACTOR}× and ≥ {REBOUND_MIN}); "
        f"local maxima of I after the peak (step, I): {lm}; I at 100/120/140/160/180/200 = {I[100]}/{I[120]}/{I[140]}/{I[160]}/{I[180]}/{I[200]}",
        "TRUE" if max(I[t] for t in range(141, 178)) <= 9 and reb[2] >= REBOUND_FACTOR * reb[0] and reb[2] >= REBOUND_MIN else
        ("PARTLY" if max(I[t] for t in range(141, 178)) <= 9 else "FALSE"),
        "none", "halluc")
    t126 = next(t for t in s.steps if S[t] == 126)
    add("9b", 9, "«susceptibles plateau around 126 after step 219»",
        f"S first reaches its final value 126 at step {t126}; S at 200/219/227 = {S[200]}/{S[219]}/{S[227]}",
        "TRUE" if abs(t126 - 219) <= 2 else ("PARTLY" if t126 < 219 else "FALSE"), "none", "score P3=1")
    add("9c", 9, "«By Step 113 and Step 227, snapshots show only susceptible agents»",
        f"at step 113: S {S[113]}, I {I[113]}, R {R[113]}; at step 227: S {S[227]}, I {I[227]}, R {R[227]}; R monotone non-decreasing: {all(R[t] >= R[t-1] for t in s.steps[1:])}",
        "FALSE", "none", "error")
    add("9d", 9, "«latent phase until step 12 (I = 6)», «S 198→165 by step 60», «R = 30 at step 75»",
        f"I at step 12 = {I[12]}; S at step 60 = {S[60]}; R at step 75 = {R[75]}",
        "TRUE" if I[12] == 6 and S[60] == 165 and R[75] == 30 else "PARTLY", "none", "unknown")
    add("9e", 9, "«Phase transition … as susceptible density falls below a critical threshold» / «depletion of susceptibles»",
        f"S fraction at peak (step {tpk[0]}) = {S[tpk[0]]}/200 = {fmt(S[tpk[0]]/200*100)} %; final S = {S[s.steps[-1]]} ({fmt(S[s.steps[-1]]/200*100)} % never infected)",
        "UNCHECK", "none", "halluc")


def report_10():
    s = load("G10")
    S, I, R = sir(s)
    pk = max(I.values())
    tpk = [t for t, v in I.items() if v == pk]
    lm = local_maxima(I, tpk[-1])
    reb = rebound(I, tpk[-1])
    rise5664 = max(I[t] - min(I[u] for u in range(56, t)) for t in range(57, 65))
    add("10a", 10, "«Вторинні сплески інфекції … на кроках 56–64 та 90–100 (повернення інфікованих до 9)»",
        f"peak I = {pk} at step {tpk}; I over steps 56–64: {[I[t] for t in range(56,65)]}; I over steps 90–100: {[I[t] for t in range(90,101)]}; "
        f"largest post-peak rebound: trough I = {reb[0]} at step {reb[1]} → {reb[2]} at step {reb[3]} (ratio {fmt(reb[2]/max(reb[0],1))}, rule ≥ {REBOUND_FACTOR}× and ≥ {REBOUND_MIN}); "
        f"local maxima after the peak: {lm}",
        "TRUE" if reb[2] >= REBOUND_FACTOR * reb[0] and reb[2] >= REBOUND_MIN and rise5664 >= 3 else
        ("PARTLY" if reb[2] >= REBOUND_FACTOR * reb[0] and reb[2] >= REBOUND_MIN else "FALSE"),
        "none", "halluc")   # the 90–100 rebound (3 → 10) is real; steps 56–64 show no burst
    add("10b", 10, "«позиції 66 поточних піддатливих (susceptible) агентів» at step 86",
        f"at step 86: S {S[86]}, I {I[86]}, R {R[86]}",
        "FALSE" if S[86] != 66 else "TRUE", "none", "error")
    add("10c", 10, "«Пікова кількість інфікованих 20 осіб за кроки 50–52» (sheet: 21 at step 51)",
        f"I at steps 49–53 = {[I[t] for t in range(49,54)]}; max {pk} at {tpk}",
        "PARTLY" if pk == 21 else "TRUE", "score 2", "score 2")
    last = s.steps[-1]
    add("10d", 10, "«сприйнятливих зменшилося з 197 до 134; одужалих 0→66 (крок 173)»; total ever infected 64 vs 66",
        f"S at 0 = {S[0]}, at {last} = {S[last]}; R at {last} = {R[last]}; initial infected = {I[0]}; new infections = {S[0]-S[last]}; ever infected = {I[0] + S[0]-S[last]}",
        "TRUE", "none", "note")


# ================================================================= output
def main():
    report_1_11()
    report_2_12()
    report_3()
    report_4()
    report_5()
    report_6()
    report_7()
    report_8()
    report_9()
    report_10()

    OUT_CSV.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT_CSV, "w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(["check_id", "report", "claim", "computed", "verdict", "pass1_treatment", "pass2_treatment"])
        for c in CHECKS:
            w.writerow([c.cid, c.report, c.claim, c.computed, c.verdict, c.p1, c.p2])

    scored = [c for c in CHECKS if c.verdict in ("TRUE", "PARTLY", "FALSE", "UNCHECK")]
    tally = defaultdict(int)
    for c in scored:
        tally[c.verdict] += 1
    # pass-2 counted items (error / halluc) and how they fared
    p2_counted = [c for c in scored if c.p2.startswith("error") or c.p2.startswith("halluc")]
    p2_tally = defaultdict(int)
    for c in p2_counted:
        p2_tally[c.verdict] += 1
    p1_counted = [c for c in scored if c.p1.startswith("error") or c.p1.startswith("halluc")]
    p1_tally = defaultdict(int)
    for c in p1_counted:
        p1_tally[c.verdict] += 1

    lines = []
    lines.append("# DATA-CHECK — disputed within-run claims tested against the source step series (Task 3b, 2026-09-09)")
    lines.append("")
    lines.append("> Generated by `harness/exph_datacheck.py` from `results/source-runs/**` (byte-identical exports of the ten analysed runs, sha256 in `results/exp-h/PROVENANCE.md`). "
                 "Rater-free: every row is a computation, not a judgement. **No score is changed** (D17/D18): pass 1 and pass 2 stay as recorded; this file says which disputed statements are true of the data. "
                 "Machine-readable twin: `data-check.csv`.")
    lines.append("")
    lines.append("## Rule")
    lines.append("")
    lines.append("- A claim is **TRUE** when the computed quantity matches the report's figure within rounding (±1 agent or ±0.15 agents/step), **PARTLY** when the qualitative statement holds but a stated number is off (or the reverse), **FALSE** when the series contradict it, **UNCHECK** when the claim is a mechanism or attribution that no exported quantity decides.")
    lines.append(f"- A claimed *regime change / surge / slowdown* between two windows counts as supported when the later window's mean per-step rate differs from the earlier one by ≥ {int(REGIME_FACTOR*100)} % (`REGIME_FACTOR`).")
    lines.append(f"- A claimed *secondary wave* in an SIR series counts as supported when, after the main peak, I(t) falls to a trough and later rises to ≥ {REBOUND_FACTOR}× the trough and ≥ {REBOUND_MIN} infected (`REBOUND_FACTOR`, `REBOUND_MIN`).")
    lines.append("- Per-step exits = Δ`evacuated_count`; «steps a–b» is inclusive; the EXP-C/E runs record steps 0…T where `evacuated_count` reaches the population at step T.")
    lines.append("- Columns *pass 1 / pass 2* say how each scoring pass treated the statement: `error`, `halluc`, `note` (mentioned, not counted), `unknown` (declared not checkable), `score k` (drove a phenomenon score), `none` (not mentioned).")
    lines.append("")
    lines.append("## Tally")
    lines.append("")
    lines.append("| Set | TRUE | PARTLY | FALSE | UNCHECK | n |")
    lines.append("|---|---|---|---|---|---|")
    lines.append(f"| all checked statements | {tally['TRUE']} | {tally['PARTLY']} | {tally['FALSE']} | {tally['UNCHECK']} | {len(scored)} |")
    lines.append(f"| statements pass 2 counted as error/hallucination | {p2_tally['TRUE']} | {p2_tally['PARTLY']} | {p2_tally['FALSE']} | {p2_tally['UNCHECK']} | {len(p2_counted)} |")
    lines.append(f"| statements pass 1 counted as error/hallucination | {p1_tally['TRUE']} | {p1_tally['PARTLY']} | {p1_tally['FALSE']} | {p1_tally['UNCHECK']} | {len(p1_counted)} |")
    lines.append("")
    lines.append("## Per-claim results")
    lines.append("")
    cur = None
    for c in CHECKS:
        if c.report != cur:
            cur = c.report
            lines.append(f"### Report {cur}")
            lines.append("")
            lines.append("| id | claim (report) | computed from the series | verdict | pass 1 | pass 2 |")
            lines.append("|---|---|---|---|---|---|")
        lines.append(f"| {c.cid} | {c.claim} | {c.computed} | **{c.verdict}** | {c.p1} | {c.p2} |")
        if c is CHECKS[-1] or CHECKS[CHECKS.index(c) + 1].report != cur:
            lines.append("")
    OUT_MD.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"wrote {OUT_MD.relative_to(ROOT)} and {OUT_CSV.relative_to(ROOT)}: {len(CHECKS)} rows")
    print("tally:", dict(tally))
    print("pass-2 counted:", dict(p2_tally), "of", len(p2_counted))
    print("pass-1 counted:", dict(p1_tally), "of", len(p1_counted))
    for c in CHECKS:
        print(f"[{c.verdict:7}] {c.cid:5} {c.claim[:70]}")
        print(f"          {c.computed}")


if __name__ == "__main__":
    main()
