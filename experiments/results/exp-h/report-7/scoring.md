# EXP-H scoring - report 7

Meta: sim_id=6a610ff2ff82d466bf2db995 analysis_id=6a63a3d14ccaa6983ac98325 lang=uk llm_target=o4-mini generated=2026-07-24T17:41:37.046Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-7.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Вогонь поширюється лісом. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "кiлькiсть горючих дерев спадала до нуля до кроку 10" |
| 2 | 2 | "знищивши 7.66 % початкових 1802 дерев (138)" |
| 3 | 2 | "пожежа не захопила весь лiс переважно через перерваний зв'язок ... самообмежене поширення" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 2
- dynamics EMERGENT 'Фронтова хвиля горiння рухається справа налiво' - right-to-left; ignition is the LEFT edge so the front moves left-to-right (the emergence section correctly describes the peak on the left moving inward)
- dynamics 'незгорiлих залишилося 1723 iз 1802 - падiння на 4,4%' conflicts with the 138 ash reported elsewhere (138/1802 = 7.66 %, the exec-summary figure)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Провести серiї симуляцiй iз рiзними щiльностями дерев (0.3-0.7) для виявлення порогу перколяцiї"

Language/format notes: uk. All 3 sub-critical phenomena correct: quick die-out (step 10), small burned fraction (7.66% all-ash vs D34 5.94% non-left - same magnitude), no burn-through/self-limited. Two secondary factual slips (front direction; 4.4% vs 138-ash inconsistency).
