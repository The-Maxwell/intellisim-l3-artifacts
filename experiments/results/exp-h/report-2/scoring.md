# EXP-H scoring - report 2

Meta: sim_id=6a5bbcc1c44d001ef3acc10e analysis_id=6a63a2a44ccaa6983ac98316 lang=uk llm_target=o4-mini generated=2026-07-24T17:36:36.53Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-2.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з одного приміщення через одні двері. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "формування черги перед вузьким отвором ... Вузькість виходу (3 клітини) -> утворення затору" |
| 2 | 2 | "Стабільна середня швидкість ~3 особи/крок спостерігалась від кроку 9 до 30" |
| 3 | 1 | "остаточна евакуація завершилась на кроці 43 (approx. 57 c)" |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'остаточна евакуація завершилась на кроцi 43 (approx. 57 c)' - the ~57 s absolute time contradicts the 0.3 s/step campaign timescale (43 steps ~= 12.9 s, ensemble 13.2 s); the step-count is correct but the seconds figure is ~4x too high

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Провести серiю прогонiв iз рiзними ширинами виходу (4-6 клiтин)"

Language/format notes: uk rendered correctly (374 Cyrillic chars, no tofu). Internal inconsistency: emergence says step 43, spatial says step 45 (both within the 44-47 data range). P3 scored 1: egress identified in steps but stated an incorrect absolute time.
