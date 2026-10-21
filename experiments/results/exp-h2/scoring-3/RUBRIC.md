# Rubric (verbatim from the pre-registered protocol, section 2)

Each report has a ground-truth sheet with exactly 3 core phenomena. Each sheet states the phenomenon, the measured value(s) with source file paths, and the acceptable range = the value's data-supported range.

Per phenomenon: `2` = explicitly identified AND quantitatively characterized correctly (numbers within the data-supported range) - `1` = partially/qualitatively identified (or quantitative but imprecise) - `0` = missed or wrongly characterized.

Per report, additionally counted and each instance quoted verbatim in the sheet:
- Factual errors - statements contradicting the exported data/statistics (as given in the sheet's ground-truth column).
- Hallucinated phenomena - asserted patterns/mechanisms with no support in the data. Hedged speculation explicitly marked as such by the report is neither an error nor a hallucination.
- Usefulness: actionable recommendations present (y/n, quote one).

Scores must cite verbatim report quotes as evidence. Score against the DATA (the sheet's measured values and data-supported ranges), not against external textbook/benchmark expectations.

Scope of "the report": everything under the `report` key of report.json (sections[].title/narrative/key_findings/causal_links/emergent_phenomena, executive_summary, conclusions, recommendations). Numbers given as approximate ("~", "approx.", "≈") are correct if within the data-supported range. A phenomenon identified but with a number outside the range is 1 (imprecise) unless the statement contradicts the data outright (then 0 and also a factual error). Internal wobbles that do not contradict the sheet's data are notes, not errors. Count each distinct erroneous claim once even if repeated.


Pass-3 addition (tagging only, scores unchanged): when a quote you credit is phrased as an expectation or hypothesis rather than as an observation of this run (e.g. "would", "is expected to", "likely", "typically"), append the tag `[expectation]` after that quote.
