"""Level-3 article figures (Task 6; inventory in article/00-article-outline.md §7).

    python harness/l3_figures.py            # all four figures
    python harness/l3_figures.py f3 f4      # a subset

Outputs: results/_article/figures/l3-fN.{pdf,png} (300 dpi) and the 1200-dpi cache
article/draft/_figcache/l3-fN-1200.png (gitignored; re-injected into the article PDF).
All labels are Ukrainian; information is carried by fills, hatches and line styles only
(journal: no colour backgrounds). Deterministic: no randomness anywhere.
"""
from __future__ import annotations

import csv
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import matplotlib.pyplot as plt  # noqa: E402
from matplotlib.patches import FancyBboxPatch, Rectangle  # noqa: E402

from figstyle_uk import COL_W, FULL_W, SCORE_FILL, save, style, uk, uk_formatter  # noqa: E402

ROOT = HERE.parent
OUT = ROOT / "results" / "_article" / "figures"
CACHE = ROOT / "article" / "draft" / "_figcache"
RUBRIC = ROOT / "results" / "exp-h-rescoring" / "rubric.csv"

LLM_HATCH = "////"
LLM_FACE = "#e6e6e6"


# ------------------------------------------------------------------ drawing helpers
def box(ax, x, y, w, h, text, *, llm=False, thick=False, dashed=False, fs=7, pad=0.02):
    face = LLM_FACE if llm else "white"
    p = FancyBboxPatch((x, y), w, h, boxstyle=f"round,pad={pad}", linewidth=1.4 if thick else 0.7,
                       edgecolor="black", facecolor=face, hatch=LLM_HATCH if llm else None,
                       linestyle="--" if dashed else "-")
    ax.add_patch(p)
    ax.text(x + w / 2, y + h / 2, text, ha="center", va="center", fontsize=fs, linespacing=1.15,
            bbox=dict(boxstyle="square,pad=0.15", facecolor="white", edgecolor="none") if llm else None)
    return p


def arrow(ax, x0, y0, x1, y1, *, dashed=False, lw=0.8):
    ax.annotate("", xy=(x1, y1), xytext=(x0, y0),
                arrowprops=dict(arrowstyle="-|>", lw=lw, color="black", shrinkA=0, shrinkB=0,
                                linestyle="--" if dashed else "-", mutation_scale=8))


def blank(ax, w, h):
    ax.set_xlim(0, w)
    ax.set_ylim(0, h)
    ax.set_aspect("equal")
    ax.axis("off")


# ------------------------------------------------------------------ Рис. 1
def fig1_dataflow():
    """Three integration levels as a data flow; Level 3 highlighted (column width)."""
    fig, ax = plt.subplots(figsize=(COL_W, COL_W * 1.36))
    W, H = 100, 136
    blank(ax, W, H)
    xl, wl = 4, 66           # left column of boxes
    y = H - 3
    rows = [
        ("Користувач: діалог природною мовою", False, False, 9),
        ("Рівень 1 – генерація моделі\nLLM перетворює опис на визначення моделі", True, False, 13),
        ("Визначення моделі: архетипи, правила\nповедінки (умови + дії), середовище", False, False, 12),
        ("Рівень 2 – виконання симуляції\nрушій правил, консультації LLM\nза обраною стратегією (необов’язково)", True, False, 17),
        ("Історія прогону: покрокові стани агентів,\nстатистика виконання (часові ряди)", False, False, 12),
        ("Рівень 3 – аналіз результатів\nпакет свідчень → модель міркування →\nструктурований звіт", True, True, 17),
        ("Звіт (PDF): резюме, явища, причинно-\nнаслідкові зв’язки, рекомендації", False, False, 12),
    ]
    centers = []
    for text, llm, thick, h in rows:
        y -= h
        box(ax, xl, y, wl, h, text, llm=llm, thick=thick, fs=6.6)
        centers.append((y, y + h))
        y -= 5
    # arrows from the bottom of box i to the top of box i+1
    for i in range(len(centers) - 1):
        bottom_i = centers[i][0]
        top_next = centers[i + 1][1]
        arrow(ax, xl + wl / 2, bottom_i, xl + wl / 2, top_next)
    # LLM provider on the right, dashed connections to the three LLM levels
    xr, wr = 76, 20
    top, bot = centers[1][1], centers[5][0]
    box(ax, xr, bot, wr, top - bot, "Постачальник\nLLM\n(мікросервіс-\nадаптер)", dashed=True, fs=6.6)
    for i in (1, 3, 5):
        yc = (centers[i][0] + centers[i][1]) / 2
        arrow(ax, xr, yc, xl + wl, yc, dashed=True, lw=0.6)
        arrow(ax, xl + wl, yc, xr, yc, dashed=True, lw=0.6)
    # legend
    ax.add_patch(Rectangle((xl, 1.5), 6, 3.2, facecolor=LLM_FACE, hatch=LLM_HATCH, edgecolor="black", lw=0.6))
    ax.text(xl + 7.5, 3.1, "участь LLM", va="center", fontsize=6.6)
    ax.add_patch(Rectangle((xl + 30, 1.5), 6, 3.2, facecolor="white", edgecolor="black", lw=0.6))
    ax.text(xl + 37.5, 3.1, "детерміновані дані", va="center", fontsize=6.6)
    return save(fig, OUT, "l3-f1", CACHE)


# ------------------------------------------------------------------ Рис. 2
def fig2_pipeline():
    """Level-3 pipeline (full width)."""
    fig, ax = plt.subplots(figsize=(FULL_W, FULL_W * 0.37))
    W, H = 200, 74
    blank(ax, W, H)
    y0, h = 28, 34
    xs = [2, 29, 64, 101, 142, 173]
    ws = [22, 30, 32, 36, 26, 25]
    texts = [
        ("Запит на аналіз\n(вікно кроків\nзафіксовано)", False),
        ("Пакет свідчень\nE = $\\langle$M, R, Σ, P, Π, S$\\rangle$\nрозріджені ряди,\n≤ 3 знімки по\n≤ 50 агентів", False),
        ("Планування секцій:\nемерджентність – завжди,\nдинаміка – типово,\nпросторова структура –\nпросторові моделі,\nподії, правила – за запитом", False),
        ("Виклики секцій\n(модель міркування)\nJSON: заголовок, наратив,\nключові спостереження,\nпричинно-наслідкові\nзв’язки, емерджентні явища", True),
        ("Синтез\n(модель міркування)\nрезюме, висновки,\nрекомендації", True),
        ("Діаграми +\nPDF-звіт →\nсховище файлів,\nподії перебігу", False),
    ]
    for x, w, (t, llm) in zip(xs, ws, texts):
        box(ax, x, y0, w, h, t, llm=llm, fs=6.2)
    for i in range(len(xs) - 1):
        arrow(ax, xs[i] + ws[i], y0 + h / 2, xs[i + 1], y0 + h / 2)
    # fail-soft side exit
    box(ax, 96, 4, 76, 14, "Відмовостійка деградація: попередження section_failed\nабо chart_skipped, звіт формується без секції\nчи діаграми, аналіз не переривається", dashed=True, fs=6.2)
    arrow(ax, xs[3] + ws[3] / 2, y0, xs[3] + ws[3] / 2, 18, dashed=True, lw=0.6)
    arrow(ax, xs[5] + ws[5] / 2, y0, xs[5] - 2, 18, dashed=True, lw=0.6)
    # top annotation
    ax.annotate("", xy=(xs[4] + ws[4], y0 + h + 4), xytext=(xs[3], y0 + h + 4),
                arrowprops=dict(arrowstyle="|-|", lw=0.6, color="black", mutation_scale=3))
    ax.text((xs[3] + xs[4] + ws[4]) / 2, y0 + h + 6.5, "в експерименті: 3 секції + 1 синтез = 4 виклики LLM на звіт",
            ha="center", va="bottom", fontsize=6.6)
    # legend
    ax.add_patch(Rectangle((2, 8), 6, 4, facecolor=LLM_FACE, hatch=LLM_HATCH, edgecolor="black", lw=0.6))
    ax.text(9.5, 10, "участь LLM", va="center", fontsize=6.6)
    ax.add_patch(Rectangle((2, 1.5), 6, 4, facecolor="white", edgecolor="black", lw=0.6))
    ax.text(9.5, 3.5, "детермінований крок", va="center", fontsize=6.6)
    return save(fig, OUT, "l3-f2", CACHE)


# ------------------------------------------------------------------ Рис. 3
SCENARIO_UA = {
    1: "евакуація з приміщення", 2: "евакуація з приміщення",
    3: "вузьке місце, тест 12", 4: "широкий отвір, тест 4",
    5: "евакуація, рішення LLM (архетип)", 6: "евакуація, рішення LLM (агент)",
    7: "лісова пожежа, p = 0,50", 8: "лісова пожежа, p = 0,70",
    9: "SIR-епідемія", 10: "SIR-епідемія",
    11: "евакуація з приміщення", 12: "евакуація з приміщення",
}


def read_rubric():
    rows = {1: {}, 2: {}}
    with open(RUBRIC, encoding="utf-8", newline="") as fh:
        for r in csv.DictReader(fh):
            rows[int(r["pass"])][int(r["report"])] = r
    assert len(rows[1]) == 12 and len(rows[2]) == 12, "rubric.csv must hold both passes"
    return rows


def fig3_scores():
    """12 reports × 3 phenomena × 2 passes score grid + error / hallucination counts (full width)."""
    rub = read_rubric()
    fig, ax = plt.subplots(figsize=(FULL_W, FULL_W * 0.50))
    n = 12
    cell_w, cell_h, pair_gap, group_gap = 1.6, 1.0, 0.12, 1.2
    x_label_w = 34.0
    # x positions of the six score cells
    xs = []
    x = x_label_w
    for k in range(3):
        xs.append(x); x += cell_w + pair_gap
        xs.append(x); x += cell_w + group_gap
    x_err = x + 0.4
    x_hal = x_err + 4.4
    W = x_hal + 4.6
    ax.set_xlim(0, W); ax.set_ylim(-1.0, 14 + cell_h + 2.9); ax.axis("off")

    def row_y(i):  # report i (1..12) → bottom y of its row; stretch pair below a 1-unit gap; 2 units kept at the bottom
        return (n - i) + (1.0 if i <= 10 else 0.0) + 2.0

    for i in range(1, 13):
        y = row_y(i)
        r1, r2 = rub[1][i], rub[2][i]
        label = f"{i:>2}  {r1['lang']}  {r1['model']}   {SCENARIO_UA[i]}"
        ax.text(0.3, y + cell_h / 2, label, va="center", ha="left", fontsize=6.8, family="serif")
        for k in range(3):
            for p, rr in ((0, r1), (1, r2)):
                sc = int(rr[f"p{k+1}"])
                face, hatch = SCORE_FILL[sc]
                ax.add_patch(Rectangle((xs[2 * k + p], y + 0.06), cell_w, cell_h - 0.12, facecolor=face,
                                       hatch=hatch, edgecolor="black", lw=0.5))
        ax.text(x_err + 1.8, y + cell_h / 2, f"{r1['errors']} / {r2['errors']}", ha="center", va="center", fontsize=6.8)
        ax.text(x_hal + 1.8, y + cell_h / 2, f"{r1['hallucinations']} / {r2['hallucinations']}", ha="center", va="center", fontsize=6.8)
    # separator between primary and stretch (label sits in the gap, on a white patch over the dotted line)
    ysep = row_y(10) - 0.5
    ax.plot([0.3, W - 0.3], [ysep, ysep], color="black", lw=0.5, ls=":")
    ax.text(W - 0.3, ysep, "звіти 11–12: повтори звітів 1–2 моделлю o3", ha="right", va="center", fontsize=6.2,
            style="italic", bbox=dict(boxstyle="square,pad=0.15", facecolor="white", edgecolor="none"))
    # headers
    ytop = row_y(1) + cell_h
    for k in range(3):
        xc = (xs[2 * k] + xs[2 * k + 1] + cell_w) / 2
        ax.text(xc, ytop + 1.55, f"Явище {k+1}", ha="center", va="bottom", fontsize=6.8)
        ax.text(xs[2 * k] + cell_w / 2, ytop + 0.25, "I", ha="center", va="bottom", fontsize=6.4)
        ax.text(xs[2 * k + 1] + cell_w / 2, ytop + 0.25, "II", ha="center", va="bottom", fontsize=6.4)
    ax.text(x_err + 1.8, ytop + 0.9, "Помилки\nI / II", ha="center", va="bottom", fontsize=6.4, linespacing=1.1)
    ax.text(x_hal + 1.8, ytop + 0.9, "Галюцин.\nI / II", ha="center", va="bottom", fontsize=6.4, linespacing=1.1)
    ax.text(0.3, ytop + 0.9, "№   мова   модель   сценарій", ha="left", va="bottom", fontsize=6.4)
    # sums
    def s(p, key, lo, hi):
        return sum(int(rub[p][i][key]) for i in range(lo, hi + 1))
    ysum = row_y(12) - 1.0
    ax.plot([0.3, W - 0.3], [row_y(12) - 0.35, row_y(12) - 0.35], color="black", lw=0.5)
    ax.text(0.3, ysum, f"Σ звітів 1–10: бали за явища {s(1,'points',1,10)}/60 (I) і {s(2,'points',1,10)}/60 (II), помилки й галюцинації →",
            ha="left", va="center", fontsize=6.6)
    ax.text(x_err + 1.8, ysum, f"{s(1,'errors',1,10)} / {s(2,'errors',1,10)}", ha="center", va="center", fontsize=6.6, weight="bold")
    ax.text(x_hal + 1.8, ysum, f"{s(1,'hallucinations',1,10)} / {s(2,'hallucinations',1,10)}", ha="center", va="center", fontsize=6.6, weight="bold")
    # legend
    ly = ysum - 1.15
    for j, (sc, lab) in enumerate(((2, "2 бали"), (1, "1 бал"), (0, "0 балів"))):
        face, hatch = SCORE_FILL[sc]
        lx = 0.3 + j * 5.2
        ax.add_patch(Rectangle((lx, ly - 0.4), 1.2, 0.8, facecolor=face, hatch=hatch, edgecolor="black", lw=0.5))
        ax.text(lx + 1.6, ly, lab, va="center", fontsize=6.4)
    ax.text(0.3 + 3 * 5.2 + 1.0, ly, "I – перший прохід (з покроковими рядами), II – другий, сліпий прохід (еталонний аркуш і звіт)",
            va="center", fontsize=6.4)
    return save(fig, OUT, "l3-f3", CACHE)


# ------------------------------------------------------------------ Рис. 4
def fig4_discharge():
    """EXP-C rep3 discharge curve with fit and the three window rates (column width)."""
    import exph_datacheck as dc  # noqa: E402  (reads results/source-runs/**)
    s = dc.load("C3")
    t0, t1, r2 = dc.linearity(s)
    steps = [t for t in s.steps if t <= t1 + 2]
    evac = [s.evac.get(t, 0) for t in steps]
    exits = [s.exits(t) for t in steps]
    windows = [(1, 13), (15, 24), (25, 45)]
    rates = [s.rate(a, b) for a, b in windows]
    # fit line over t0-1..t1 (same as linearity())
    xs = list(range(t0 - 1, t1 + 1)); ys = [s.evac[t] for t in xs]
    n = len(xs); mx, my = sum(xs) / n, sum(ys) / n
    slope = sum((x - mx) * (y - my) for x, y in zip(xs, ys)) / sum((x - mx) ** 2 for x in xs)
    icpt = my - slope * mx

    fig, (a1, a2) = plt.subplots(2, 1, sharex=True, figsize=(COL_W, COL_W * 0.95), height_ratios=[1.6, 1])
    for k, (a, b) in enumerate(windows):
        for ax in (a1, a2):
            ax.axvspan(a - 0.5, b + 0.5, facecolor="#e9e9e9" if k % 2 == 0 else "#f6f6f6", edgecolor="none", zorder=0)
    a1.step(steps, evac, where="post", color="black", lw=1.0, label="евакуйовано (виміряно)")
    a1.plot(xs, [slope * x + icpt for x in xs], color="black", lw=0.8, ls="--",
            label=f"лінійна апроксимація, R² = {uk(r2, 2)}")
    a1.set_ylabel("Евакуйовано, осіб")
    a1.set_ylim(0, 105)
    a1.legend(loc="lower right", frameon=False)
    a1.yaxis.set_major_formatter(uk_formatter(0))
    a2.bar(steps, exits, width=0.8, color="#7a7a7a", edgecolor="none", label="вихід за крок")
    for (a, b), r in zip(windows, rates):
        a2.plot([a - 0.5, b + 0.5], [r, r], color="black", lw=1.2)
        a2.text((a + b) / 2, r + 0.35, f"{uk(r, 2)}/крок", ha="center", va="bottom", fontsize=6.6)
    a2.set_ylim(0, 6.6)
    a2.set_ylabel("Вихід, осіб/крок")
    a2.set_xlabel("Крок симуляції")
    a2.set_xlim(-0.5, t1 + 2.5)
    a2.xaxis.set_major_formatter(uk_formatter(0))
    a2.yaxis.set_major_formatter(uk_formatter(0))
    return save(fig, OUT, "l3-f4", CACHE)


FIGURES = {"f1": fig1_dataflow, "f2": fig2_pipeline, "f3": fig3_scores, "f4": fig4_discharge}


def main() -> int:
    names = sys.argv[1:] or list(FIGURES)
    style()
    for n in names:
        for p in FIGURES[n]():
            print("wrote", p.relative_to(ROOT))
    return 0


if __name__ == "__main__":
    sys.exit(main())
