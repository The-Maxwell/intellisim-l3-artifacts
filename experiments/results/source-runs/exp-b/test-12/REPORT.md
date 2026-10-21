# EXP-B Test 12 (stretch) — bottleneck congestion localization — REPORT

Protocol: `32-rimea-protocol.md` §6. Date: 2026-07-18. Engine: core `525dca0`. Model
`exp-B12-bottleneck` (`6a5bc236c44d001ef3af6848`): Room 1 (150 persons, 10x10 m) -> 1.2 m door ->
2 m corridor -> Room 2 -> 1.2 m exit. 10 reps, gen seeds 1201-1210 (`raw/`).

## Results

| Metric | Zone 1 (upstream Room-1 door) | Zone 2 (upstream Room-2 exit) |
|---|---|---|
| Peak occupancy (mean over reps) | **37.2** (35-40) | **12.7** (12-14) |
| Steps with occupancy >= 10 (mean) | ~34.5 | ~16.6 |

Ratio of mean peaks **2.9**; the two distributions do not overlap (zone-1 min 35 > zone-2 max 14)
— sustained congestion forms only upstream, in every rep. Drainage: 147/150 evacuate per rep;
exactly 3 agents wedge at the same geometric pinch points every rep (2 at Room-1 door edges,
1 at corridor mouth) -> all runs end `max_steps` (documented straggler class; the qualitative
criterion is unaffected).

## Verdict

**PASS (qualitative, as pre-registered):** peak zone-1 occupancy >> zone-2 (2.9x, no overlap) —
congestion localizes at the upstream bottleneck, per RiMEA T12's documentary intent.
