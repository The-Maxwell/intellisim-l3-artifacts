"""Shared matplotlib style for the Level-3 article figures (Task 6, D8).

Journal constraints («Управління розвитком складних систем»): Times New Roman text,
two 8.0 cm columns with a 16.5 cm text width, no colour backgrounds, every symbol
explained in the caption. The style is black-and-white first: information is carried
by black/gray fills, hatches, markers and line styles — never by hue.

Usage:
    from figstyle_uk import style, save, COL_W, FULL_W, uk, uk_formatter
    style(); fig, ax = plt.subplots(figsize=(COL_W, 0.75 * COL_W)); ...; save(fig, OUT, "l3-f1")
"""
from __future__ import annotations

from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402
from matplotlib.ticker import FuncFormatter  # noqa: E402

MM = 1 / 25.4
COL_W = 80 * MM      # one column (8.0 cm)
FULL_W = 165 * MM    # full text width (16.5 cm)
HIRES_DPI = 1200     # cache raster re-injected into the PDF by fix_pdf_images.py

# Black-and-white-first palette: fills and lines ordered from darkest to lightest.
BLACK = "#000000"
GRAYS = ["#000000", "#555555", "#999999", "#cccccc", "#ffffff"]
HATCHES = ["", "////", "....", "xxxx", "\\\\\\\\"]
MARKERS = ["o", "s", "^", "D", "v"]
LINES = ["-", "--", "-.", ":"]

# Score cells (Рис. 3): 2 = black, 1 = hatched light gray, 0 = white.
SCORE_FILL = {2: ("#000000", ""), 1: ("#d9d9d9", "////"), 0: ("#ffffff", "")}


def style() -> None:
    """Apply the article style (Times New Roman with Cyrillic, STIX math, TrueType embedding)."""
    plt.rcParams.update({
        "font.family": "serif",
        "font.serif": ["Times New Roman", "DejaVu Serif"],
        "mathtext.fontset": "stix",
        "pdf.fonttype": 42,
        "ps.fonttype": 42,
        "figure.dpi": 100,
        "savefig.dpi": 300,
        "font.size": 8,
        "axes.labelsize": 8,
        "axes.titlesize": 8,
        "xtick.labelsize": 7,
        "ytick.labelsize": 7,
        "legend.fontsize": 7,
        "axes.spines.top": False,
        "axes.spines.right": False,
        "axes.linewidth": 0.6,
        "lines.linewidth": 1.0,
        "hatch.linewidth": 0.5,
        "figure.facecolor": "white",
        "axes.facecolor": "white",
        "savefig.facecolor": "white",
        "figure.constrained_layout.use": True,
        "axes.unicode_minus": False,
    })


def uk(x: float, nd: int = 1) -> str:
    """Ukrainian number: decimal comma, thin thousands space, no trailing '.0' for integers."""
    if nd == 0 or float(x).is_integer() and nd <= 1:
        s = f"{int(round(x)):,}".replace(",", " ")
        return s
    s = f"{x:,.{nd}f}".replace(",", " ").replace(".", ",")
    return s


def uk_formatter(nd: int = 0) -> FuncFormatter:
    return FuncFormatter(lambda v, _pos: uk(v, nd))


def save(fig, out_dir: Path, basename: str, cache_dir: Path | None = None) -> list[Path]:
    """PDF (submission) + 300-dpi PNG (draft/visual check) + optional 1200-dpi cache PNG."""
    out_dir.mkdir(parents=True, exist_ok=True)
    written = []
    for ext in ("pdf", "png"):
        p = out_dir / f"{basename}.{ext}"
        fig.savefig(p, metadata={"Creator": "harness/l3_figures.py"} if ext == "pdf" else None)
        written.append(p)
    if cache_dir is not None:
        cache_dir.mkdir(parents=True, exist_ok=True)
        p = cache_dir / f"{basename}-{HIRES_DPI}.png"
        fig.savefig(p, dpi=HIRES_DPI)
        written.append(p)
    plt.close(fig)
    return written
