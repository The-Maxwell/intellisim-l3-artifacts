# EXP-H scoring - report 10

Meta: sim_id=6a636c0c4ccaa6983ab1976a analysis_id=6a63a46f4ccaa6983ac9832e lang=uk llm_target=o4-mini generated=2026-07-24T17:44:15.641Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-10.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Інфекційне захворювання поширюється в популяції рухомих агентів. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "швидке зростання числа iнфiкованих до пiку в 20 агентiв на кроках 50-52 (spatial: peak 21)" |
| 2 | 2 | "усi iнфiкованi (66 осiб) одужали ... сприйнятливих залишилося 134 (final size 66/200 = 0.33)" |
| 3 | 2 | "Одужалих агентiв наростало з 0 до 66 ... сприйнятливих зменшилося з 197 до 134 (monotone)" |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Варiювати тривалiсть infection_timer i порiг активацiї infected_recovers"

Language/format notes: uk rendered correctly. Peak ~20-21 @ 50-52 (GT 21 @ 51); final size 33% (66 recovered) matches. Trivial internal wobbles (peak 20 vs 21; 64 vs 66 infected) do not contradict the data.
