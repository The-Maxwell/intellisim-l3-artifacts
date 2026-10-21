# EXP-H scoring - report 12

Meta: sim_id=6a5bbcc1c44d001ef3acc10e analysis_id=6a63a4fe4ccaa6983ac98334 lang=uk llm_target=o3 generated=2026-07-24T17:46:38.068Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-2.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з одного приміщення через одні двері. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o3"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Агентам довелось самоорганiзуватися у дворядну чергу в зонi x = 33-40, y = 18-22 ... ~2,2-2,9 особи/крок" |
| 2 | 2 | "мiж 8-м i 34-м кроками утримувався квазiсталий потiк 2-4 особи/крок" |
| 3 | 2 | "повна евакуацiя завершилась на 45-му кроцi (approx. 13,5 c) ... один крок - це ... approx. 0,30 с" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Перезапустити модель iз ширшими дверима (>=2,4 м, 6 клiтин) або кiлькома отворами"

Language/format notes: STRETCH (o3), uk, same sim as report 2. CORRECTLY converts 0.3 s/step -> 13.5 s (where the o4-mini report 2 fabricated ~57 s) and infers 0.4 m/cell (2688.8 cells ~= 1.07 km; 38 cells = 15.2 m). uk rendered correctly.
