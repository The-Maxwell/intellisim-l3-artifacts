"""Citation-list gate for the Level-3 article (Task 4, D19).

Parses the summary table of ``article/04-citation-register.md`` and checks the
journal's reference rules for the *core* list (column ``Core`` = ``✓``):

* 10 <= N <= 20
* self-citations <= 20 % of N
* foreign Scopus/WoS-indexed items published 2021-2026 (flag ``RFI``) >= 50 % of N
* every row that is core has status VERIFIED

Flags column vocabulary: ``self`` (author's own), ``ua`` (Ukrainian-language source),
``RFI`` (recent foreign indexed), ``preprint`` (arXiv/DataCite DOI only), ``web`` (no DOI,
URL + access date), ``old`` (foreign, before 2021).

Usage:  python harness/citation_gate.py            # gate on the core list
        python harness/citation_gate.py --with KEY1,KEY2   # what if these reserves are added
Exit code 1 when a rule fails.
"""
import re
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
ROOT = Path(__file__).resolve().parents[1]
REG = ROOT / "article" / "04-citation-register.md"
YEAR_NOW = 2026
RECENT_FROM = YEAR_NOW - 5  # «за останні 5 років» → 2021..2026


def parse_rows(text):
    rows = []
    in_table = False
    for line in text.splitlines():
        if line.startswith("| Key |"):
            in_table = True
            continue
        if in_table:
            if not line.startswith("|"):
                break
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if len(cells) < 7 or set(cells[0]) <= {"-"}:
                continue
            key, record, doi, year, flags, core, status = cells[:7]
            rows.append(
                {
                    "key": key.strip("`"),
                    "year": int(re.search(r"\d{4}", year).group()) if re.search(r"\d{4}", year) else None,
                    "flags": {f.strip() for f in flags.split(",") if f.strip() and f.strip() != "—"},
                    "core": core.strip() == "✓",
                    "status": status,
                    "doi": doi,
                }
            )
    return rows


def gate(rows, extra=()):
    core = [r for r in rows if r["core"] or r["key"] in extra]
    n = len(core)
    self_n = sum("self" in r["flags"] for r in core)
    rfi = [r for r in core if "RFI" in r["flags"]]
    unverified = [r["key"] for r in core if not r["status"].startswith("VERIFIED")]
    bad_rfi = [r["key"] for r in rfi if r["year"] is None or r["year"] < RECENT_FROM or "self" in r["flags"] or "preprint" in r["flags"]]
    no_doi = [r["key"] for r in core if "web" in r["flags"]]
    ok = True
    print(f"core list: N = {n}  (rule 10 ≤ N ≤ 20) -> {'OK' if 10 <= n <= 20 else 'FAIL'}")
    ok &= 10 <= n <= 20
    print(f"self-citations: {self_n}/{n} = {100*self_n/n:.1f} %  (rule ≤ 20 %) -> {'OK' if self_n/n <= 0.20 else 'FAIL'}")
    ok &= self_n / n <= 0.20
    print(f"recent foreign Scopus/WoS ({RECENT_FROM}–{YEAR_NOW}): {len(rfi)}/{n} = {100*len(rfi)/n:.1f} %  (rule ≥ 50 %) -> {'OK' if len(rfi)/n >= 0.50 else 'FAIL'}")
    ok &= len(rfi) / n >= 0.50
    print(f"RFI rows failing their own flag (year/self/preprint): {bad_rfi or 'none'} -> {'OK' if not bad_rfi else 'FAIL'}")
    ok &= not bad_rfi
    print(f"unverified core rows: {unverified or 'none'} -> {'OK' if not unverified else 'FAIL'}")
    ok &= not unverified
    print(f"core rows without DOI (URL-only web sources): {no_doi or 'none'}  (journal: DOI for every digital source — keep at zero or justify)")
    print("core keys:", ", ".join(r["key"] for r in core))
    return ok


def main():
    extra = ()
    if len(sys.argv) > 2 and sys.argv[1] == "--with":
        extra = tuple(k.strip() for k in sys.argv[2].split(","))
    rows = parse_rows(REG.read_text(encoding="utf-8"))
    if not rows:
        print("no rows parsed from", REG)
        sys.exit(2)
    print(f"register rows parsed: {len(rows)}")
    ok = gate(rows, extra)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
