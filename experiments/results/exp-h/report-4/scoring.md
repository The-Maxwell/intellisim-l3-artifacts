# EXP-H scoring - report 4

Meta: sim_id=6a5bc463c44d001ef3b8f824 analysis_id=6a63a3444ccaa6983ac9831c lang=uk llm_target=o4-mini generated=2026-07-24T17:39:16.482Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-4.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди проходять крізь широкий отвір за різної щільності натовпу. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "297 за 0-90 кроків ... walk_to_opening домiнувало (12 198 запускiв)" |
| 2 | 1 | "Пропускна здатнiсть нелiнiйно залежить вiд щiльностi: максимум ~6,5 агентiв/крок при середнiй концентрацiї ~160 осiб" |
| 3 | 0 | "(no support for absence-of-saturation; the report instead asserts density-limited congestion)" |

**Phenomenon points: 3/6**

Factual errors (verbatim + what the data says): count = 2
- 'Висока початкова густина (300 агентiв) -> прискорений вихiд' combined with '0-15 крокiв: вихiд 43 осiб через високi затори' - attributes low early throughput to high-density congestion at the opening; at step 0-15 agents are still travelling to the opening (the engine has no speed-density coupling, per EXP-B/C)
- spatial section: '50 випадково вибраних агентiв рiвномiрно розподiленi' at Step 0 while the run is N=300 (the dynamics section correctly uses 300) - population misstatement

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Пропускна здатнiсть нелiнiйно залежить вiд щiльностi: максимум ~6,5 агентiв/крок при середнiй концентрацiї ~160 осiб' - a fundamental-diagram peak inferred from a single fixed-N run; the intermediate 'maximum' is really the post-arrival window, not a density-flow optimum

Actionable recommendations: y - quote: "Варiювати початкову щiльнiсть (150-450 агентiв) для мапування повної дiаграми залежностi пропускної здатностi вiд концентрацiї"

Language/format notes: uk. The HARDEST single-run case: the T4 fundamental-diagram truth is an ensemble result (N50/150/300) not observable in one N=300 run, and the run's within-time flow profile is dominated by travel-time-to-opening. The report over-interprets it as a density-flow optimum and mis-attributes low early flow to congestion. Honest weak report - published as data, not a protocol failure (protocol section 3).
