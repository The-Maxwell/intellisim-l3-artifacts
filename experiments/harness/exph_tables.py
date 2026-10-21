"""Paste-ready article tables for the Level-3 manuscript (Task 6; inventory in
article/00-article-outline.md §8).

    python harness/exph_tables.py     → results/_article/tables/TABLES.md

Табл. 1 (source runs) is transcribed from results/source-runs/MANIFEST.md / the
ground-truth sheets and kept here as data with its provenance; Табл. 2 and Табл. 3 are
computed from results/exp-h-rescoring/rubric.csv, data-check.csv and
results/exp-h/manifest.json and asserted against results/_article/KEY-NUMBERS.md.
Output blocks use the builder directives of Task 7 (`@tabcap … | width= | key=` then
`@table: size=` and a markdown table). Ukrainian typography: decimal comma, nbsp before
units and %, «I / II» = pass 1 / pass 2.
"""
from __future__ import annotations

import csv
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
ROOT = Path(__file__).resolve().parents[1]
RUBRIC = ROOT / "results" / "exp-h-rescoring" / "rubric.csv"
DATACHECK = ROOT / "results" / "exp-h-rescoring" / "data-check.csv"
MANIFEST = ROOT / "results" / "exp-h" / "manifest.json"
OUT = ROOT / "results" / "_article" / "tables" / "TABLES.md"
NB = " "

# --- constants of record (KEY-NUMBERS §5, D13); asserted below where recomputable
COST_TOTAL_USD = 1.0269          # exph-cost.md §Cost of record
COST_PER_REPORT_USD = COST_TOTAL_USD / 12
EXPECTED = dict(points1=54, points2=46, errors1=7, errors2=11, hall1=1, hall2=9,
                kappa30=0.437, kappa30w=0.487, kappa36=0.422, kappa36w=0.472,
                dc_true=6, dc_partly=10, dc_false=8, dc_uncheck=1)


def uk(x: float, nd: int = 1) -> str:
    if nd == 0:
        return f"{int(round(x)):,}".replace(",", NB)
    return f"{x:,.{nd}f}".replace(",", NB).replace(".", ",")


# ------------------------------------------------------------------ Табл. 1 (transcribed; provenance MANIFEST.md)
TAB1 = [
    (1, "евакуація з приміщення: 100 агентів, один вихід (повтор 3)", "en",
     "черга біля виходу; ≈ 2,2 агента/крок, майже лінійне спорожнення; повна евакуація за 45 кроків (13,5 с)", "EXP-C"),
    (2, "евакуація з приміщення: той самий сценарій (повтор 18)", "uk",
     "≈ 2,3 агента/крок; майже лінійне спорожнення; 44 кроки (13,2 с)", "EXP-C"),
    (3, "вузьке місце (тест 12 RiMEA): 150 агентів, дві кімнати та коридор", "en",
     "затор перед звуженням (пік зайнятості зони 1 — 37,2 проти 12,7 у зоні 2); 147/150 вийшли до ≈ кроку 98; 3 агенти застрягли", "EXP-B"),
    (4, "широкий отвір (тест 4 RiMEA): отвір 4 м, 300 агентів", "uk",
     "297/300 вийшли до кроку 83 (≈ 3,6 агента/крок); питомий потік зростає з щільністю; насичення понад ≈ 2 ос./м² не спостерігається (ансамблева властивість)", "EXP-B"),
    (5, "евакуація з рішеннями LLM на рівні архетипу (повтор 301)", "en",
     "98 кроків проти 46 без LLM; час виходу агента 10,8 ± 6,4 с проти 6,7 ± 3,3 с; мітки рішень 441 / 178 / 55 / 40", "EXP-E"),
    (6, "евакуація з рішеннями LLM для кожного агента (повтор 301)", "uk",
     "46 кроків — як без LLM; час виходу 6,76 ± 3,25 с; усі 494 рішення — «прямувати до виходу»", "EXP-E"),
    (7, "лісова пожежа, щільність дерев p = 0,50 (докритичний режим)", "uk",
     "згасання до кроку 10; вигоріло 5,9 % дерев; фронт не досяг протилежного краю", "EXP-F"),
    (8, "лісова пожежа, p = 0,70 (надкритичний режим)", "en",
     "вигоріло 99,25 % дерев; фронт пройшов ґратку; 85 кроків", "EXP-F"),
    (9, "просторова SIR-епідемія: 200 агентів, β = 0,18 (повтор 1)", "en",
     "єдиний пік I = 22 на кроці 60; кінцевий розмір 74/200 = 0,37; монотонні S↓ та R↑", "EXP-G"),
    (10, "просторова SIR-епідемія: той самий сценарій (повтор 8)", "uk",
     "єдиний пік I = 21 на кроці 51; кінцевий розмір 66/200 = 0,33; монотонні S↓ та R↑", "EXP-G"),
]


def table1() -> str:
    # v6 (D31, T16 amended): no experiment codes in the article — the «Джерело» column is not printed;
    # the code stays in TAB1 as provenance (report → campaign experiment → MANIFEST.md).
    lines = [
        "@tabcap: Вихідні прогони та виміряні еталонні явища (три на звіт), зафіксовані до генерації звітів | width=full | key=tab_runs",
        "@table: size=9",
        "| № звіту | Сценарій (сімейство, розмір) | Мова | Виміряні еталонні явища |",
        "|---|---|---|---|",
    ]
    for n, sc, lang, phen, src in TAB1:
        lines.append(f"| {n} | {sc} | {lang} | {phen} |")
    lines.append("")
    lines.append("Примітка до табл. 1: звіти 11 і 12 — повтори звітів 1 і 2 моделлю o3 (звіти 1–10 — o4-mini). "
                 "Провенанс (код експерименту кампанії рівнів 1–2, з експортів якого виміряно еталон): " +
                 ", ".join(f"{n} — {src}" for n, _, _, _, src in TAB1) + ". "
                 "Транскрибовано з `results/source-runs/MANIFEST.md` (аркуші `results/exp-h/ground-truth/sheet-N.md`).")
    return "\n".join(lines)


# ------------------------------------------------------------------ rubric
def read_rubric():
    rows = {1: {}, 2: {}}
    with open(RUBRIC, encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            rows[int(r["pass"])][int(r["report"])] = r
    assert len(rows[1]) == 12 == len(rows[2])
    return rows


def ssum(rub, p, key, lo, hi):
    return sum(int(rub[p][i][key]) for i in range(lo, hi + 1))


def read_rubric3_orig():
    """Pass-3 rows of the primary series (arm «orig», reports 1–10) from results/exp-h2/rubric3.csv (protocol 37)."""
    rows = {}
    with (ROOT / "results" / "exp-h2" / "rubric3.csv").open(encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            if r["arm"] == "orig":
                rows[int(r["report"])] = r
    assert sorted(rows) == list(range(1, 11)), sorted(rows)
    out = json.loads((ROOT / "results" / "exp-h2" / "OUTCOMES.json").read_text(encoding="utf-8"))
    for k in ("points", "errors", "hallucinations"):   # asserted against OUTCOMES.json / KEY-NUMBERS §7b
        assert sum(int(rows[n][k]) for n in rows) == out["arms"]["orig"][k], k
    assert sum(int(rows[n]["points"]) for n in rows) == 40
    return rows


def table2(rub) -> str:
    """Табл. 2 (v6, D31): scores of all three passes per report; pass III (protocol 37, blind, sheet-only)
    exists for the primary series only, so reports 11–12 print «–». The all-«так» recommendation flag
    moved to the text (10/10 in every pass)."""
    r3 = read_rubric3_orig()
    lines = [
        "@tabcap: Оцінки дванадцяти звітів за рубрикою у трьох проходах оцінювання (I – перший, II і III – сліпі) | width=full | key=tab_scores",
        "@table: size=9",
        "| № | Мова | Модель | Явище 1, I / II / III | Явище 2, I / II / III | Явище 3, I / II / III | Сума балів, I / II / III | Помилки, I / II / III | Галюциновані явища, I / II / III |",
        "|---|---|---|---|---|---|---|---|---|",
    ]
    def tri(a, b, c, key):
        return f"{a[key]} / {b[key]} / {c[key] if c else '–'}"
    for i in range(1, 13):
        a, b, c = rub[1][i], rub[2][i], r3.get(i)
        lines.append(f"| {i} | {a['lang']} | {a['model']} | {tri(a,b,c,'p1')} | {tri(a,b,c,'p2')} | {tri(a,b,c,'p3')} | "
                     f"{tri(a,b,c,'points')} | {tri(a,b,c,'errors')} | {tri(a,b,c,'hallucinations')} |")
        if i == 10:
            s3 = lambda key: sum(int(r3[n][key]) for n in range(1, 11))
            lines.append(f"| Разом | | | {ssum(rub,1,'p1',1,10)} / {ssum(rub,2,'p1',1,10)} / {s3('p1')} | "
                         f"{ssum(rub,1,'p2',1,10)} / {ssum(rub,2,'p2',1,10)} / {s3('p2')} | "
                         f"{ssum(rub,1,'p3',1,10)} / {ssum(rub,2,'p3',1,10)} / {s3('p3')} | "
                         f"**{ssum(rub,1,'points',1,10)} / {ssum(rub,2,'points',1,10)} / {s3('points')}** | "
                         f"**{ssum(rub,1,'errors',1,10)} / {ssum(rub,2,'errors',1,10)} / {s3('errors')}** | "
                         f"**{ssum(rub,1,'hallucinations',1,10)} / {ssum(rub,2,'hallucinations',1,10)} / {s3('hallucinations')}** |")
    lines.append("")
    lines.append("Примітка до табл. 2: бали за явище — 2 (назване і кількісно правильне в межах підтвердженого даними діапазону), 1 (часткове, якісне або неточне), 0 (пропущене чи хибне); "
                 "звіти 11–12 — повтори звітів 1–2 моделлю o3, до критеріїв не входять і в третьому проході не оцінювалися. "
                 "Джерела: `results/exp-h-rescoring/rubric.csv` (I, II), `results/exp-h2/rubric3.csv` (III, арм «orig»).")
    return "\n".join(lines)


# ------------------------------------------------------------------ κ (cross-check of kappa.md)
def kappa(a, b, weighted=False):
    cats = sorted(set(a) | set(b))
    n = len(a)
    idx = {c: i for i, c in enumerate(cats)}
    m = [[0] * len(cats) for _ in cats]
    for x, y in zip(a, b):
        m[idx[x]][idx[y]] += 1
    rows = [sum(r) for r in m]; cols = [sum(c) for c in zip(*m)]
    k = len(cats)
    if not weighted:
        po = sum(m[i][i] for i in range(k)) / n
        pe = sum(rows[i] * cols[i] for i in range(k)) / n / n
        return (po - pe) / (1 - pe)
    w = [[1 - abs(i - j) / (k - 1) for j in range(k)] for i in range(k)]
    po = sum(w[i][j] * m[i][j] for i in range(k) for j in range(k)) / n
    pe = sum(w[i][j] * rows[i] * cols[j] for i in range(k) for j in range(k)) / n / n
    return (po - pe) / (1 - pe)


def items(rub, lo, hi):
    a, b = [], []
    for i in range(lo, hi + 1):
        for key in ("p1", "p2", "p3"):
            a.append(int(rub[1][i][key])); b.append(int(rub[2][i][key]))
    return a, b


# ------------------------------------------------------------------ data-check tally
def datacheck_tally():
    t = {"TRUE": 0, "PARTLY": 0, "FALSE": 0, "UNCHECK": 0}
    with open(DATACHECK, encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            if r["pass2_treatment"].startswith(("error", "halluc")):  # incl. «error (attribution)», «halluc (mechanism)»
                t[r["verdict"]] += 1
    return t


def latency():
    m = json.loads(MANIFEST.read_text(encoding="utf-8"))
    lat = [r["token_usage"]["total"]["latency_ms"] / 1000 for r in m["reports"]]
    return min(lat), max(lat), sum(lat) / len(lat)


def table3(rub) -> str:
    a30, b30 = items(rub, 1, 10); a36, b36 = items(rub, 1, 12)
    k30, k30w = kappa(a30, b30), kappa(a30, b30, True)
    k36, k36w = kappa(a36, b36), kappa(a36, b36, True)
    agree30 = sum(x == y for x, y in zip(a30, b30)) / len(a30)
    p1, p2 = ssum(rub, 1, "points", 1, 10), ssum(rub, 2, "points", 1, 10)
    e1, e2 = ssum(rub, 1, "errors", 1, 10), ssum(rub, 2, "errors", 1, 10)
    h1, h2 = ssum(rub, 1, "hallucinations", 1, 10), ssum(rub, 2, "hallucinations", 1, 10)
    perf = [sum(int(rub[p][i]["points"]) == 6 for i in range(1, 11)) for p in (1, 2)]
    ge5 = [sum(int(rub[p][i]["points"]) >= 5 for i in range(1, 11)) for p in (1, 2)]
    en = [i for i in range(1, 11) if rub[1][i]["lang"] == "en"]; ukr = [i for i in range(1, 11) if rub[1][i]["lang"] == "uk"]
    pts = lambda p, ids: sum(int(rub[p][i]["points"]) for i in ids)
    err = lambda p, ids: sum(int(rub[p][i]["errors"]) for i in ids)
    tally = datacheck_tally()
    lmin, lmax, lmean = latency()
    # assertions against KEY-NUMBERS
    assert (p1, p2, e1, e2, h1, h2) == (EXPECTED["points1"], EXPECTED["points2"], EXPECTED["errors1"], EXPECTED["errors2"], EXPECTED["hall1"], EXPECTED["hall2"]), (p1, p2, e1, e2, h1, h2)
    assert abs(k30 - EXPECTED["kappa30"]) < 0.001 and abs(k30w - EXPECTED["kappa30w"]) < 0.001, (k30, k30w)
    assert abs(k36 - EXPECTED["kappa36"]) < 0.001 and abs(k36w - EXPECTED["kappa36w"]) < 0.001, (k36, k36w)
    assert (tally["TRUE"], tally["PARTLY"], tally["FALSE"], tally["UNCHECK"]) == (EXPECTED["dc_true"], EXPECTED["dc_partly"], EXPECTED["dc_false"], EXPECTED["dc_uncheck"]), tally
    assert abs(lmin - 28.8) < 0.1 and abs(lmax - 60.1) < 0.1 and abs(lmean - 44.8) < 0.1, (lmin, lmax, lmean)

    rows = [
        ("Бали за передреєстровані явища (звіти 1–10)", "≥ 42/60 (70 %)", f"{p1}/60 ({uk(p1/60*100,0)}{NB}%) / {p2}/60 ({uk(p2/60*100,0)}{NB}%)"),
        ("Критерій 1", "", "виконано / виконано"),
        ("Бездоганні звіти (6/6); звіти ≥ 5/6", "—", f"{perf[0]} з 10; {ge5[0]} з 10 / {perf[1]} з 10; {ge5[1]} з 10"),
        ("Фактичні помилки, всього (у середньому на звіт)", "≤ 10 (≤ 1)", f"{e1} ({uk(e1/10,1)}) / {e2} ({uk(e2/10,1)})"),
        ("Критерій 2", "", "виконано / не виконано"),
        ("Галюциновані явища", "описово", f"{h1} / {h2}"),
        ("Звіти з рекомендаціями", "описово", "10 з 10 / 10 з 10"),
        ("Бали en / uk (по 5 звітів)", "описово", f"{pts(1,en)} і {pts(1,ukr)} з 30 / {pts(2,en)} і {pts(2,ukr)} з 30"),
        ("Помилки en / uk", "описово", f"{err(1,en)} і {err(1,ukr)} / {err(2,en)} і {err(2,ukr)}"),
        ("Пара o4-mini (1, 2) — бали / помилки", "описово", f"{pts(1,[1,2])}/12, {err(1,[1,2])} / {pts(2,[1,2])}/12, {err(2,[1,2])}"),
        ("Пара o3 (11, 12) — бали / помилки", "описово", f"{pts(1,[11,12])}/12, {err(1,[11,12])} / {pts(2,[11,12])}/12, {err(2,[11,12])}"),
        ("κ Коена між проходами, 30 позицій (лінійно зважений)", "—", f"{uk(k30,2)} ({uk(k30w,2)}); точний збіг {uk(agree30*100,0)}{NB}%"),
        ("Розходження між проходами", "—", "9 з 30 позицій, усі 2 → 1"),
        ("Перевірка спірних тверджень за даними (25 позицій проходу II)", "—",
         f"підтверджено {tally['TRUE']}; спростовано {tally['FALSE']}; числово неточні {tally['PARTLY']}; не перевіряється {tally['UNCHECK']}"),
        ("Вартість за обліком постачальника (12 звітів; на звіт)", "—", f"{uk(COST_TOTAL_USD,2)} дол. США; ≈ {uk(COST_PER_REPORT_USD,2)} дол."),
        ("Тривалість аналізу на звіт; виклики LLM", "—", f"{uk(lmin,0)}–{uk(lmax,0)}{NB}с (середнє {uk(lmean,0)}{NB}с); 4"),
    ]
    lines = [
        "@tabcap: Критерії прийняття та агреговані показники за двома проходами оцінювання (I / II) | width=col | key=tab_criteria",
        "@table: size=9",
        "| Показник | Поріг | Значення, I / II |",
        "|---|---|---|",
    ]
    for r in rows:
        lines.append(f"| {r[0]} | {r[1]} | {r[2]} |")
    lines.append("")
    lines.append("Примітка до табл. 3: обидва проходи виконано за однією рубрикою; перший мав доступ до покрокових рядів, другий — лише до еталонного аркуша та звіту. "
                 "Перевірка за даними — діагностика рубрики, а не третій прохід (D18). Вартість — рядки обліку постачальника за 24.07.2026 × прейскурантні ціни (перевірено 08.09.2026). "
                 "Джерела: `rubric.csv`, `kappa.md`, `data-check.csv`, `results/exp-h/manifest.json`, `results/_billing/exph-cost.md`.")
    return "\n".join(lines)


def table_h2() -> str:
    """EXP-H2 pass 3 (D24, protocol 37): points per run for the three arms + totals; manuscript v4 «Таблиця 3»."""
    rows = {}
    with (ROOT / "results" / "exp-h2" / "rubric3.csv").open(encoding="utf-8") as fh:
        for r in csv.DictReader(fh):
            rows[(r["arm"], int(r["report"]))] = r
    out = json.loads((ROOT / "results" / "exp-h2" / "OUTCOMES.json").read_text(encoding="utf-8"))
    arms = ("orig", "F2", "A")
    tot = {a: {k: sum(int(rows[(a, n)][k]) for n in range(1, 11)) for k in ("points", "errors", "hallucinations")} for a in arms}
    for a in arms:   # asserted against OUTCOMES.json and KEY-NUMBERS §10
        assert tot[a]["points"] == out["arms"][a]["points"], a
        assert tot[a]["errors"] == out["arms"][a]["errors"], a
        assert tot[a]["hallucinations"] == out["arms"][a]["hallucinations"], a
    assert [tot[a]["points"] for a in arms] == [40, 44, 14]
    assert out["primary"]["sign_test"]["positive"] == 10
    lines = ["@tabcap: Оцінки трьох груп звітів у третьому, сліпому проході за прогонами звітів 1–10 (бали з 6, суми за 10 звітів) | width=col | key=tab_h2",
             "@table: size=9",
             "| Прогін звіту | Основна серія | Повторна генерація | Без даних прогону |",
             "|---|---|---|---|"]
    for n in range(1, 11):
        lines.append(f"| {n} | " + " | ".join(rows[(a, n)]["points"] for a in arms) + " |")
    lines.append("| Сума балів | " + " | ".join(f"**{tot[a]['points']}**" for a in arms) + " |")
    lines.append("| Помилки | " + " | ".join(str(tot[a]["errors"]) for a in arms) + " |")
    lines.append("| Галюциновані явища | " + " | ".join(str(tot[a]["hallucinations"]) for a in arms) + " |")
    lines += ["", "Примітка: основна серія – звіти 1–10 від 24.07.2026; повторна генерація – ті самі запити до рушія 23.09.2026; "
              "без даних прогону – пакет свідчень скорочено до зведень моделі й прогону, згенеровано поза метасистемою 23.09.2026 "
              "(протокол 37, D24). Джерела: `results/exp-h2/rubric3.csv`, `OUTCOMES.json`."]
    return "\n".join(lines)


def main() -> int:
    rub = read_rubric()
    parts = [
        "# TABLES — paste-ready @table blocks for the Level-3 manuscript",
        "",
        "> Generated by `harness/exph_tables.py` from `rubric.csv`, `data-check.csv`, `manifest.json` (+ Табл. 1 transcribed from `MANIFEST.md`). "
        "Values are asserted against `results/_article/KEY-NUMBERS.md` at generation time. Do not edit by hand — edit the script. "
        "Directive syntax per PLAN.md «Manuscript directives» (Task 7 builder). Non-breaking spaces are real U+00A0 characters.",
        "",
        "## Табл. 1", "", table1(), "",
        "## Табл. 2", "", table2(rub), "",
        "## Табл. 3 (v1 inventory — NOT in the manuscript; its cost/latency rows are superseded by KEY-NUMBERS §7b)", "", table3(rub), "",
        "## Табл. 3 (manuscript v4) — EXP-H2, pass 3", "", table_h2(), "",
    ]
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text("\n".join(parts) + "\n", encoding="utf-8")
    print("wrote", OUT.relative_to(ROOT))
    return 0


if __name__ == "__main__":
    sys.exit(main())
